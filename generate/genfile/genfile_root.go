package genfile

import (
	"sort"

	"github.com/mineiros-io/terramate/config"
	"github.com/mineiros-io/terramate/errors"
	"github.com/mineiros-io/terramate/hcl/eval"
	"github.com/mineiros-io/terramate/lets"
	"github.com/mineiros-io/terramate/project"
	"github.com/zclconf/go-cty/cty"
)

// LoadStackContext loads and parses from the file system all generate_file
// blocks for a given stack. It will navigate the file system from the stack dir
// until it reaches rootdir, loading generate_file blocks found on Terramate
// configuration files.
//
// All generate_file blocks must have unique labels, even ones at different
// directories. Any conflicts will be reported as an error.
//
// Metadata and globals for the stack are used on the evaluation of the
// generate_file blocks.
//
// The rootdir MUST be an absolute path.
func LoadRootContext(
	cfg *config.Tree,
	projmeta project.Metadata,
) ([]File, error) {
	genFileBlocks := cfg.DownwardGenerateFiles()
	var files []File

	for _, genFileBlock := range genFileBlocks {
		name := genFileBlock.Label

		evalctx, err := eval.NewContext(cfg.RootDir())
		if err != nil {
			panic(err)
		}

		evalctx.SetNamespace("terramate", projmeta.ToCtyMap())

		context := "stack"
		if genFileBlock.Context != nil {
			val, err := evalctx.Eval(genFileBlock.Context.Expr)
			if err != nil {
				return nil, errors.E(
					genFileBlock.Range,
					err,
					"failed to evaluate genfile context",
				)
			}
			if val.Type() != cty.String {
				return nil, errors.E(
					"generate_file.context must be a string but given %s",
					val.Type().FriendlyName(),
				)
			}
			context = val.AsString()
		}

		// only handle stack context here.
		if context != "root" {
			continue
		}

		err = lets.Load(genFileBlock.Lets, evalctx)
		if err != nil {
			return nil, err
		}

		condition := true
		if genFileBlock.Condition != nil {
			value, err := evalctx.Eval(genFileBlock.Condition.Expr)
			if err != nil {
				return nil, errors.E(ErrConditionEval, err)
			}
			if value.Type() != cty.Bool {
				return nil, errors.E(
					ErrInvalidConditionType,
					"condition has type %s but must be boolean",
					value.Type().FriendlyName(),
				)
			}
			condition = value.True()
		}

		if !condition {
			files = append(files, File{
				label:     name,
				origin:    genFileBlock.Range,
				condition: condition,
				context:   context,
			})
			continue
		}

		asserts := make([]config.Assert, len(genFileBlock.Asserts))
		assertsErrs := errors.L()
		assertFailed := false

		for i, assertCfg := range genFileBlock.Asserts {
			assert, err := config.EvalAssert(evalctx, assertCfg)
			if err != nil {
				assertsErrs.Append(err)
				continue
			}
			asserts[i] = assert
			if !assert.Assertion && !assert.Warning {
				assertFailed = true
			}
		}

		if err := assertsErrs.AsError(); err != nil {
			return nil, err
		}

		if assertFailed {
			files = append(files, File{
				label:     name,
				origin:    genFileBlock.Range,
				condition: condition,
				asserts:   asserts,
			})
			continue
		}

		value, err := evalctx.Eval(genFileBlock.Content.Expr)
		if err != nil {
			return nil, errors.E(ErrContentEval, err)
		}

		if value.Type() != cty.String {
			return nil, errors.E(
				ErrInvalidContentType,
				"content has type %s but must be string",
				value.Type().FriendlyName(),
			)
		}

		files = append(files, File{
			label:     name,
			origin:    genFileBlock.Range,
			body:      value.AsString(),
			condition: condition,
			asserts:   asserts,
		})
	}

	sort.Slice(files, func(i, j int) bool {
		return files[i].String() < files[j].String()
	})

	return files, nil
}
