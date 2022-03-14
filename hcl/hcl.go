// Copyright 2021 Mineiros GmbH
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package hcl

import (
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclparse"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/hashicorp/hcl/v2/hclwrite"
	"github.com/madlambda/spells/errutil"
	"github.com/mineiros-io/terramate/hcl/eval"
	"github.com/rs/zerolog/log"
	"github.com/zclconf/go-cty/cty"
)

// Module represents a terraform module.
// Note that only the fields relevant for terramate are declared here.
type Module struct {
	Source string // Source is the module source path (eg.: directory, git path, etc).
}

type Config struct {
	// absdir is the absolute path to the configuration directory.
	absdir    string
	Terramate *Terramate
	Stack     *Stack
}

type GitConfig struct {
	DefaultBranchBaseRef string // DefaultBranchBaseRef is the baseRef when in default branch.
	DefaultBranch        string // DefaultBranch is the default branch.
	DefaultRemote        string // DefaultRemote is the default remote.
}

// GenerateConfig represents code generation config
type GenerateConfig struct {
	LocalsFilename     string
	BackendCfgFilename string
}

type RootConfig struct {
	Git      *GitConfig
	Generate *GenerateConfig
}

// Terramate is the parsed "terramate" HCL block.
type Terramate struct {
	// RequiredVersion contains the terramate version required by the stack.
	RequiredVersion string

	// RootConfig is the configuration at the project root directory (commonly
	// the git directory).
	RootConfig *RootConfig

	Backend *hclsyntax.Block
}

// Stack is the parsed "stack" HCL block.
type Stack struct {
	// Name of the stack
	Name string

	// Description of the stack
	Description string

	// After is a list of non-duplicated stack entries that must run before the
	// current stack runs.
	After []string

	// Before is a list of non-duplicated stack entries that must run after the
	// current stack runs.
	Before []string

	// Wants is a list of non-duplicated stack entries that must be selected
	// whenever the current stack is selected.
	Wants []string
}

// HCLBlocks maps a filename to a slice of blocks associated with it
type HCLBlocks map[string][]*hclsyntax.Block

const (
	ErrHCLSyntax                errutil.Error = "HCL syntax error"
	ErrMalformedTerramateConfig errutil.Error = "malformed terramate config"
	ErrMalformedTerraform       errutil.Error = "malformed terraform"
)

// NewConfig creates a new HCL config with dir as config directory path.
func NewConfig(dir string) (Config, error) {
	st, err := os.Stat(dir)
	if err != nil {
		return Config{}, fmt.Errorf("initializing config: %w", err)
	}

	if !st.IsDir() {
		return Config{}, fmt.Errorf("config constructor requires a directory path")
	}

	return Config{
		absdir: dir,
	}, nil
}

// AbsDir returns the absolute path of the configuration directory.
func (c Config) AbsDir() string { return c.absdir }

// IsEmpty returns true if the config is empty, false otherwise.
func (c Config) IsEmpty() bool {
	return c.Stack == nil && c.Terramate == nil
}

// Save the configuration file using filename inside config directory.
func (c Config) Save(filename string) (err error) {
	cfgpath := filepath.Join(c.absdir, filename)
	f, err := os.Create(cfgpath)
	if err != nil {
		return fmt.Errorf("saving configuration file %q: %w", cfgpath, err)
	}

	defer func() {
		err2 := f.Close()

		if err != nil {
			return
		}

		err = err2
	}()

	return PrintConfig(f, c)
}

// NewTerramate creates a new TerramateBlock with reqversion.
func NewTerramate(reqversion string) *Terramate {
	return &Terramate{
		RequiredVersion: reqversion,
	}
}

// ParseModules parses blocks of type "module" containing a single label.
func ParseModules(path string) ([]Module, error) {
	logger := log.With().
		Str("action", "ParseModules()").
		Str("path", path).
		Logger()

	logger.Trace().
		Msg("Get path information.")
	_, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("stat failed on %q: %w", path, err)
	}

	logger.Trace().
		Msg("Create new parser.")
	p := hclparse.NewParser()

	logger.Debug().
		Msg("Parse HCL file.")
	f, diags := p.ParseHCLFile(path)
	if diags.HasErrors() {
		return nil, errutil.Chain(
			ErrHCLSyntax,
			fmt.Errorf("parsing modules: %w", diags),
		)
	}

	body, _ := f.Body.(*hclsyntax.Body)

	logger.Trace().
		Msg("Parse modules.")
	var modules []Module
	for _, block := range body.Blocks {
		if block.Type != "module" {
			continue
		}

		if len(block.Labels) != 1 {
			return nil, errutil.Chain(
				ErrMalformedTerraform,
				fmt.Errorf("module block must have 1 label"),
			)
		}

		moduleName := block.Labels[0]

		logger.Trace().
			Msg("Get source attribute.")
		source, ok, err := findStringAttr(block, "source")
		if err != nil {
			return nil, errutil.Chain(
				ErrMalformedTerraform,
				fmt.Errorf("looking for module.%q.source attribute: %w",
					moduleName, err),
			)
		}
		if !ok {
			return nil, errutil.Chain(
				ErrMalformedTerraform,
				errors.New("module must have a \"source\" attribute"),
			)
		}

		modules = append(modules, Module{Source: source})
	}

	return modules, nil
}

// ParseDir will parse Terramate configuration from a given directory,
// parsing all files with the suffixes .tm and .tm.hcl.
// Note: it does not recurse into child directories.
func ParseDir(dir string) (Config, error) {
	logger := log.With().
		Str("action", "ParseDir()").
		Str("dir", dir).
		Logger()

	logger.Trace().Msg("Parsing configuration files")

	loadedParser, err := loadCfgBlocks(dir)
	if err != nil {
		return Config{}, fmt.Errorf("parsing config files: %w", err)
	}

	logger.Trace().Msg("creating config from loaded parser")

	return newCfgFromParsedHCLs(dir, loadedParser)
}

// ParseGlobalsBlocks parses all Terramate files on the given dir, returning
// only global blocks (other blocks are discarded).
func ParseGlobalsBlocks(dir string) (HCLBlocks, error) {
	logger := log.With().
		Str("action", "ParseGlobalsBlocks").
		Str("configdir", dir).
		Logger()

	logger.Trace().Msg("loading config")

	return parseHCLBlocks(dir, "globals", func(block *hclsyntax.Block) error {
		// Not validated with schema because cant find a way to validate
		// N arbitrary attributes (defined by user/dynamic).
		if len(block.Body.Blocks) > 0 {
			return errors.New("blocks inside globals are not allowed")
		}
		if len(block.Labels) > 0 {
			return fmt.Errorf("labels on globals block are not allowed, found %v", block.Labels)
		}
		return nil
	})
}

// ParseExportAsLocalsBlocks parses all Terramate files on the given dir, returning
// only export_as_locals blocks (other blocks are discarded).
// export_as_locals blocks are validated, so the caller can expect valid blocks only or an error.
func ParseExportAsLocalsBlocks(dir string) (HCLBlocks, error) {
	logger := log.With().
		Str("action", "hcl.ParseExportAsLocalsBlocks").
		Str("configdir", dir).
		Logger()

	logger.Trace().Msg("loading config")

	return parseHCLBlocks(dir, "export_as_locals", func(block *hclsyntax.Block) error {
		if len(block.Labels) != 0 {
			return fmt.Errorf(
				"exported_as_locals should not have labels but has %v",
				block.Labels,
			)
		}
		if len(block.Body.Blocks) != 0 {
			return errors.New("export_as_locals should not have blocks")
		}
		return nil
	})
}

// ParseGenerateHCLBlocks parses all Terramate files on the given dir, returning
// only generate_hcl blocks (other blocks are discarded).
// generate_hcl blocks are validated, so the caller can expect valid blocks only or an error.
func ParseGenerateHCLBlocks(dir string) (HCLBlocks, error) {
	logger := log.With().
		Str("action", "hcl.ParseGenerateHCLBlocks").
		Str("configdir", dir).
		Logger()

	logger.Trace().Msg("loading config")

	schema := &hcl.BodySchema{
		Attributes: []hcl.AttributeSchema{},
		Blocks: []hcl.BlockHeaderSchema{
			{
				Type:       "content",
				LabelNames: []string{},
			},
		},
	}

	return parseHCLBlocks(dir, "generate_hcl", func(block *hclsyntax.Block) error {
		// Don't seem like I can use hcl.BodySchema to check for any non-empty
		// label, only specific label values.
		if len(block.Labels) != 1 {
			return fmt.Errorf(
				"generate_hcl must have single label instead got %v",
				block.Labels,
			)
		}
		if block.Labels[0] == "" {
			return errors.New("generate_hcl label can't be empty")
		}
		// Schema check passes if no block is present, so check for amount of blocks
		if len(block.Body.Blocks) != 1 {
			return fmt.Errorf("generate_hcl must have one 'content' block, got %d blocks", len(block.Body.Blocks))
		}
		_, diags := block.Body.Content(schema)
		if diags.HasErrors() {
			return diags
		}
		return nil
	})
}

// CopyBody will copy the src body to the given target, evaluating attributes using the
// given evaluation context.
//
// Scoped traversals, like name.traverse, for unknown namespaces will be copied
// as is (original expression form, no evaluation).
//
// Returns an error if the evaluation fails.
func CopyBody(stackpath string, target *hclwrite.Body, src *hclsyntax.Body, evalctx *eval.Context) error {
	logger := log.With().
		Str("action", "CopyBody()").
		Logger()

	logger.Trace().Msg("Sorting attributes.")

	// Avoid generating code with random attr order (map iteration is random)
	attrs := sortedAttributes(src.Attributes)

	for _, attr := range attrs {
		logger := logger.With().
			Str("attrName", attr.Name).
			Logger()

		logger.Trace().Msg("evaluating.")

		b, err := ioutil.ReadFile(attr.SrcRange.Filename)
		if err != nil {
			return err
		}

		exprContent := b[attr.Expr.Range().Start.Byte:attr.Expr.Range().End.Byte]
		stokens, diags := hclsyntax.LexExpression(exprContent, attr.SrcRange.Filename, hcl.Pos{})

		if diags.HasErrors() {
			return diags
		}

		tokens := ToWriteTokens(stokens)

		tokens, err = PartialEval(attr.SrcRange.Filename, tokens, evalctx)
		if err != nil {
			return err
		}

		/*val, err := evalctx.Eval(attr.Expr)
		if err != nil {
			logger.Trace().Msg("evaluation failed, checking if is unknown scope traversal.")

			scope, isUnknownTraversal := getUnknownScopeTraversal(attr.Expr, evalctx)
			if isUnknownTraversal {
				logger.Trace().Msg("Is unknown scope traversal, copying as traversal.")
				target.SetAttributeTraversal(attr.Name, scope.Traversal)
				continue
			}

			return fmt.Errorf("parsing attribute %q: %v", attr.Name, err)
		}*/

		logger.Trace().
			Str("attribute", attr.Name).
			Msg("Setting evaluated attribute.")

		target.SetAttributeRaw(attr.Name, tokens)
	}

	logger.Trace().Msg("Append blocks.")

	for _, block := range src.Blocks {
		targetBlock := target.AppendNewBlock(block.Type, block.Labels)
		if block.Body == nil {
			continue
		}
		if err := CopyBody(stackpath, targetBlock.Body(), block.Body, evalctx); err != nil {
			return err
		}
	}

	return nil
}

func ToWriteTokens(in hclsyntax.Tokens) hclwrite.Tokens {
	tokens := make([]*hclwrite.Token, len(in))
	for i, st := range in {
		tokens[i] = &hclwrite.Token{
			Type:  st.Type,
			Bytes: st.Bytes,
		}
	}
	return tokens
}

func evalArgs(fname string, tokens hclwrite.Tokens, evalctx *eval.Context) (hclwrite.Tokens, int, error) {
	pos := 0

	out := hclwrite.Tokens{}
	var next *hclwrite.Token

	for pos < len(tokens) {
		tok := tokens[pos]

		if tok.Type == hclsyntax.TokenOQuote {
			evaluated, skip, err := partialEvalString(fname, tokens[pos:], evalctx)
			if err != nil {
				return nil, 0, err
			}

			pos += skip

			out = append(out, evaluated...)
			goto nextArg
		}

		if tok.Type == hclsyntax.TokenCParen {
			out = append(out, tok)
			pos++
			break
		}

		if len(tokens[pos:]) < 2 {
			return tokens, pos, nil // TODO(i4k): review this
		}

		if tok.Type != hclsyntax.TokenIdent {
			return nil, 0, fmt.Errorf("unexpected token 2: %s '%s' (%s)", tokens[pos-1].Bytes, tok.Bytes, tok.Type)
		}

		next = tokens[pos+1]
		if next.Type == hclsyntax.TokenDot {
			evaluated, skipVar, err := evalOneVar(fname, tokens[pos:], evalctx)
			if err != nil {
				return nil, 0, err
			}

			//fmt.Printf("skipping %d tokens\n", skip)

			pos += skipVar - 1

			out = append(out, evaluated...)
		} else if next.Type == hclsyntax.TokenOParen {
			out = append(out, tok, next)
			pos += 2
			continue
		}

	nextArg:

		pos++
		if pos >= len(tokens) {
			break
		}

		tok = tokens[pos]
		if tok.Type != hclsyntax.TokenCParen &&
			tok.Type != hclsyntax.TokenComma && tok.Type != hclsyntax.TokenEOF {
			return out, pos - 1, nil
		}

		out = append(out, tok)

		if tok.Type == hclsyntax.TokenCParen {
			return out, pos, nil
		}

		pos++
	}

	return out, pos, nil
}

func interpTokenStart() *hclwrite.Token {
	return &hclwrite.Token{
		Type:  hclsyntax.TokenTemplateInterp,
		Bytes: []byte("${"),
	}
}

func interpTokenEnd() *hclwrite.Token {
	return &hclwrite.Token{
		Type:  hclsyntax.TokenTemplateSeqEnd,
		Bytes: []byte("}"),
	}
}

func countVarParts(tokens hclwrite.Tokens) int {
	count := 0
	for i := 0; i < len(tokens); i++ {
		if tokens[i].Type != hclsyntax.TokenIdent && tokens[i].Type != hclsyntax.TokenDot {
			return count
		}
		count++
	}

	return count
}

func evalOneVar(fname string, tokens hclwrite.Tokens, evalctx *eval.Context) (hclwrite.Tokens, int, error) {
	out := hclwrite.Tokens{}

	if len(tokens) < 3 {
		return nil, 0, fmt.Errorf("expected a.b but got %d tokens", len(tokens))
	}

	varLen := countVarParts(tokens)

	if string(tokens[0].Bytes) != "global" &&
		string(tokens[0].Bytes) != "terramate" {
		out = append(out, tokens...)

		return out, varLen, nil
	}

	var expr []byte

	for _, et := range tokens[:varLen] {
		expr = append(expr, et.Bytes...)
	}

	e, diags := hclsyntax.ParseExpression(expr, fname, hcl.Pos{})
	if diags.HasErrors() {
		return nil, 0, fmt.Errorf("failed to parse expr: %v", diags.Error())
	}

	val, err := evalctx.Eval(e)
	if err != nil {
		return nil, 0, err
	}

	newtoks := hclwrite.TokensForValue(val)

	out = append(out, newtoks...)

	return out, varLen, nil
}

func partialEvalString(fname string, tokens hclwrite.Tokens, evalctx *eval.Context) (hclwrite.Tokens, int, error) {
	if len(tokens) == 1 {
		panic("HCL parser should have caught this syntax error")
	}

	out := hclwrite.Tokens{}

	out = append(out, tokens[0])

	pos := 1
	didEval := false
	hasPrevQuoteLit := false

	for pos < len(tokens) {
		tok := tokens[pos]
		switch tok.Type {
		case hclsyntax.TokenCQuote:
			out = append(out, tok)
			return out, pos, nil
		case hclsyntax.TokenQuotedLit:
			out = append(out, tok)
			hasPrevQuoteLit = true
			//fmt.Printf("quoted lit: %s\n", tok.Bytes)
		case hclsyntax.TokenTemplateSeqEnd:
			if !didEval {
				out = append(out, tok)
			}
			didEval = false
		case hclsyntax.TokenTemplateInterp:
			pos++

			evaluated, skipArgs, err := evalArgs(fname, tokens[pos:], evalctx)
			if err != nil {
				return nil, 0, err
			}

			if string(evaluated[0].Bytes) != string(tokens[pos].Bytes) {
				didEval = true

				if hasPrevQuoteLit {
					str := out[len(out)-1]
					switch evaluated[0].Type {
					case hclsyntax.TokenOQuote:
						if evaluated[1].Type != hclsyntax.TokenQuotedLit {
							panic(fmt.Errorf("unexpected type %s", evaluated[1].Type))
						}

						str.Bytes = append(str.Bytes, evaluated[1].Bytes...)
					default:
						panic(fmt.Sprintf("%s (%s)", evaluated[0].Bytes, evaluated[0].Type))
					}
				} else {
					out = append(out, evaluated...)
				}
			} else {
				out = append(out, interpTokenStart())
				out = append(out, evaluated...)
			}

			pos += skipArgs

		default:
			panic(fmt.Errorf("unexpected token here %s %s (%s)", tokens[pos-1].Bytes, tok.Bytes, tok.Type))
		}

		pos++
	}

	return out, pos - 1, nil
}

func PartialEval(fname string, tokens hclwrite.Tokens, evalctx *eval.Context) (hclwrite.Tokens, error) {
	pos := 0
	out := hclwrite.Tokens{}
	for pos < len(tokens) {
		evaluated, skip, err := evalExpr(fname, tokens[pos:], evalctx)
		if err != nil {
			return nil, err
		}

		pos += skip
		out = append(out, evaluated...)
	}

	if pos < len(tokens) {
		panic(fmt.Errorf("failed to evaluate all tokens: %d != %d", pos, len(tokens)))
	}

	return out, nil
}

func evalExpr(fname string, tokens hclwrite.Tokens, evalctx *eval.Context) (hclwrite.Tokens, int, error) {
	if len(tokens) == 0 {
		return tokens, 0, nil
	}

	pos := 0

	out := hclwrite.Tokens{}

	for pos < len(tokens) {
		tok := tokens[pos]
		switch tok.Type {
		case hclsyntax.TokenEOF:
			pos += 1
		case hclsyntax.TokenOQuote:
			evaluated, skip, err := partialEvalString(fname, tokens[pos:], evalctx)
			if err != nil {
				return nil, 0, err
			}

			out = append(out, evaluated...)
			pos += skip
		case hclsyntax.TokenIdent:
			switch string(tok.Bytes) {
			case "true", "false", "null":
				out = append(out, tok)
			default:
				evaluated, skip, err := evalArgs(fname, tokens[pos:], evalctx)
				if err != nil {
					return nil, 0, err
				}

				pos += skip
				out = append(out, evaluated...)
			}
		case hclsyntax.TokenCBrack:
			out = append(out, tok)
		case hclsyntax.TokenOBrack:
			out = append(out, tok)
		case hclsyntax.TokenComma:
			out = append(out, tok)
		case hclsyntax.TokenNumberLit:
			out = append(out, tok)
		case hclsyntax.TokenOBrace:
			out = append(out, tok)
		case hclsyntax.TokenCBrace:
			out = append(out, tok)
		case hclsyntax.TokenNewline:
			out = append(out, tok)
		case hclsyntax.TokenEqual:
			out = append(out, tok)
		default:
			panic(fmt.Errorf("not implemented: %s (%s)", tok.Bytes, tok.Type))
		}

		pos++
	}

	return out, pos, nil
}

func sortedAttributes(attrs hclsyntax.Attributes) []*hclsyntax.Attribute {
	names := make([]string, 0, len(attrs))

	for name := range attrs {
		names = append(names, name)
	}

	log.Trace().
		Str("action", "sortedAttributes()").
		Msg("Sort attributes.")
	sort.Strings(names)

	sorted := make([]*hclsyntax.Attribute, len(names))
	for i, name := range names {
		sorted[i] = attrs[name]
	}

	return sorted
}

func findStringAttr(block *hclsyntax.Block, attr string) (string, bool, error) {
	logger := log.With().
		Str("action", "findStringAttr()").
		Logger()

	logger.Trace().
		Msg("Range over attributes.")
	for name, value := range block.Body.Attributes {
		if name != attr {
			continue
		}

		logger.Trace().
			Msg("Found attribute that we were looking for.")

		logger.Trace().
			Msg("Get attribute value.")
		attrVal, diags := value.Expr.Value(nil)
		if diags.HasErrors() {
			return "", false, fmt.Errorf("failed to evaluate %q attribute: %w",
				attr, diags)
		}

		logger.Trace().
			Msg("Check value type is correct.")
		if attrVal.Type() != cty.String {
			return "", false, fmt.Errorf("attribute %q is not a string", attr)
		}

		return attrVal.AsString(), true, nil
	}

	return "", false, nil
}

func assignSet(name string, target *[]string, val cty.Value) error {
	logger := log.With().
		Str("action", "assignSet()").
		Logger()

	logger.Trace().
		Msg("Check val is correct type.")
	if val.Type().IsSetType() {
		return fmt.Errorf("attribute %q is not a set", name)
	}

	logger.Trace().
		Msg("Iterate over values.")
	values := map[string]struct{}{}
	iterator := val.ElementIterator()
	for iterator.Next() {
		_, elem := iterator.Element()

		logger.Trace().
			Msg("Check element is of correct type.")
		if elem.Type() != cty.String {
			return fmt.Errorf("field %q is a set(string) but contains %q",
				name, elem.Type().FriendlyName())
		}

		logger.Trace().Msg("Get element as string.")

		str := elem.AsString()
		if _, ok := values[str]; ok {
			return fmt.Errorf("duplicated entry %q in field %q of type set(string)",
				str, name)
		}
		values[str] = struct{}{}
	}

	var elems []string
	for v := range values {
		elems = append(elems, v)
	}

	logger.Trace().Msg("Sort elements.")

	sort.Strings(elems)
	*target = elems
	return nil
}

func parseStack(stack *Stack, stackblock *hclsyntax.Block) error {
	logger := log.With().
		Str("action", "parseStack()").
		Str("stack", stack.Name).
		Logger()

	logger.Debug().Msg("Get stack attributes.")

	for name, value := range stackblock.Body.Attributes {
		logger.Trace().Msg("Get attribute value.")

		attrVal, diags := value.Expr.Value(nil)
		if diags.HasErrors() {
			return fmt.Errorf("failed to evaluate %q attribute: %w", name, diags)
		}

		logger.Trace().
			Str("attribute", name).
			Msg("Setting attribute on configuration.")

		switch name {

		case "name":
			if attrVal.Type() != cty.String {
				return fmt.Errorf("field stack.\"name\" must be a \"string\" but given %q",
					attrVal.Type().FriendlyName())
			}
			stack.Name = attrVal.AsString()

		case "after":
			err := assignSet(name, &stack.After, attrVal)
			if err != nil {
				return err
			}

		case "before":
			err := assignSet(name, &stack.Before, attrVal)
			if err != nil {
				return err
			}

		case "wants":
			err := assignSet(name, &stack.Wants, attrVal)
			if err != nil {
				return err
			}

		case "description":
			logger.Trace().Msg("parsing stack description.")
			if attrVal.Type() != cty.String {
				return fmt.Errorf("field stack.\"description\" must be a \"string\" but given %q",
					attrVal.Type().FriendlyName())
			}
			stack.Description = attrVal.AsString()

		default:
			return fmt.Errorf("unrecognized attribute stack.%q", name)
		}
	}

	return nil
}

func parseRootConfig(cfg *RootConfig, block *hclsyntax.Block) error {
	logger := log.With().
		Str("action", "parseRootConfig()").
		Logger()

	if len(block.Labels) != 0 {
		return fmt.Errorf("config type expects 0 label but has %v", block.Labels)
	}

	logger.Trace().Msg("Range over block attributes.")

	for name := range block.Body.Attributes {
		return fmt.Errorf("unrecognized attribute terramate.config.%s", name)
	}

	logger.Trace().Msg("Range over blocks.")

	for _, b := range block.Body.Blocks {
		switch b.Type {
		case "git":
			logger.Trace().Msg("Type was 'git'.")

			if cfg.Git != nil {
				return errors.New("multiple terramate.config.git blocks")
			}

			cfg.Git = &GitConfig{}

			logger.Trace().Msg("Parse git config.")

			if err := parseGitConfig(cfg.Git, b); err != nil {
				return err
			}
		case "generate":
			logger.Trace().Msg("Found block generate")

			if cfg.Generate != nil {
				return errors.New("multiple terramate.config.backend blocks")
			}

			logger.Trace().Msg("Parsing terramate.config.generate.")

			gencfg, err := parseGenerateConfig(b)
			if err != nil {
				return err
			}
			cfg.Generate = &gencfg

		default:
			return fmt.Errorf("unrecognized block type %q", b.Type)
		}
	}

	return nil
}

func parseGenerateConfig(block *hclsyntax.Block) (GenerateConfig, error) {
	logger := log.With().
		Str("action", "parseGenerateConfig()").
		Logger()

	logger.Trace().Msg("Range over block attributes.")

	cfg := GenerateConfig{}

	for name, value := range block.Body.Attributes {
		logger := logger.With().
			Str("attribute", name).
			Logger()

		attrVal, diags := value.Expr.Value(nil)
		if diags.HasErrors() {
			return GenerateConfig{}, errutil.Chain(
				ErrHCLSyntax,
				fmt.Errorf("failed to evaluate terramate.config.generate.%s attribute: %w",
					name, diags),
			)
		}

		switch name {
		case "backend_config_filename":
			{
				logger.Trace().Msg("parsing")

				if attrVal.Type() != cty.String {
					return GenerateConfig{}, fmt.Errorf("terramate.config.generate.%s is not a string but %q",
						name,
						attrVal.Type().FriendlyName())
				}
				cfg.BackendCfgFilename = attrVal.AsString()

				logger.Trace().Msg("parsed with success")
			}
		case "locals_filename":
			{
				logger.Trace().Msg("parsing")

				if attrVal.Type() != cty.String {
					return GenerateConfig{}, fmt.Errorf("terramate.config.generate.%s is not a string but %q",
						name,
						attrVal.Type().FriendlyName())
				}
				cfg.LocalsFilename = attrVal.AsString()

				logger.Trace().Msg("parsed with success")
			}

		default:
			return GenerateConfig{}, fmt.Errorf("unrecognized attribute terramate.config.generate.%s", name)
		}
	}

	if cfg.LocalsFilename != "" && cfg.LocalsFilename == cfg.BackendCfgFilename {
		return GenerateConfig{}, fmt.Errorf(
			"terramate.config.generate: locals and backend cfg files have the same name %q", cfg.LocalsFilename)
	}

	return cfg, nil
}

func parseGitConfig(git *GitConfig, block *hclsyntax.Block) error {
	logger := log.With().
		Str("action", "parseGitConfig()").
		Logger()

	logger.Trace().Msg("Range over block attributes.")

	for name, value := range block.Body.Attributes {
		attrVal, diags := value.Expr.Value(nil)
		if diags.HasErrors() {
			return fmt.Errorf("failed to evaluate terramate.config.%s attribute: %w", name, diags)
		}
		switch name {
		case "default_branch":
			logger.Trace().Msg("Attribute name was 'default_branch'.")

			if attrVal.Type() != cty.String {
				return fmt.Errorf("terramate.config.git.branch is not a string but %q",
					attrVal.Type().FriendlyName())
			}

			git.DefaultBranch = attrVal.AsString()
		case "default_remote":
			logger.Trace().Msg("Attribute name was 'default_remote'.")

			if attrVal.Type() != cty.String {
				return fmt.Errorf("terramate.config.git.remote is not a string but %q",
					attrVal.Type().FriendlyName())
			}

			git.DefaultRemote = attrVal.AsString()

		case "default_branch_base_ref":
			logger.Trace().Msg("Attribute name was 'default_branch_base_ref.")

			if attrVal.Type() != cty.String {
				return fmt.Errorf("terramate.config.git.defaultBranchBaseRef is not a string but %q",
					attrVal.Type().FriendlyName())
			}

			git.DefaultBranchBaseRef = attrVal.AsString()

		default:
			return fmt.Errorf("unrecognized attribute terramate.config.git.%s", name)
		}
	}
	return nil
}

func filterBlocksByType(blocktype string, blocks []*hclsyntax.Block) []*hclsyntax.Block {
	logger := log.With().
		Str("action", "filterBlocksByType()").
		Logger()

	var filtered []*hclsyntax.Block

	logger.Trace().Msg("Range over blocks.")

	for _, block := range blocks {
		if block.Type != blocktype {
			continue
		}

		filtered = append(filtered, block)
	}

	return filtered
}

func blockIsAllowed(name string) bool {
	logger := log.With().
		Str("action", "blockIsAllowed()").
		Logger()

	switch name {
	case "terramate", "stack", "backend", "globals", "export_as_locals", "generate_hcl":
		logger.Trace().Msg("Block name was allowed.")
		return true
	default:
		return false
	}
}

func loadCfgBlocks(dir string) (*hclparse.Parser, error) {
	logger := log.With().
		Str("action", "loadCfgBlocks()").
		Str("dir", dir).
		Logger()

	logger.Trace().Msg("listing files")

	dirEntries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("reading dir to load config files: %v", err)
	}

	logger.Trace().Msg("looking for Terramate files")

	parser := hclparse.NewParser()

	for _, dirEntry := range dirEntries {
		logger := logger.With().
			Str("entryName", dirEntry.Name()).
			Logger()

		if dirEntry.IsDir() {
			logger.Trace().Msg("ignoring dir")
			continue
		}

		filename := dirEntry.Name()
		if strings.HasSuffix(filename, ".tm") || strings.HasSuffix(filename, ".tm.hcl") {
			path := filepath.Join(dir, filename)

			logger.Trace().Msg("Reading config file.")

			data, err := os.ReadFile(path)
			if err != nil {
				return nil, fmt.Errorf("reading config file %q: %v", path, err)
			}

			logger.Trace().Msg("Parsing config.")

			_, diags := parser.ParseHCL(data, path)
			if diags.HasErrors() {
				return nil, errutil.Chain(ErrHCLSyntax, diags)
			}

			logger.Trace().Msg("Config file parsed successfully")
		}
	}

	return parser, nil
}

func newCfgFromParsedHCLs(dir string, parser *hclparse.Parser) (Config, error) {
	logger := log.With().
		Str("action", "newCfgFromParsedHCLs()").
		Str("dir", dir).
		Logger()

	tmconfig := Config{
		absdir: dir,
	}

	for fname, hclfile := range parser.Files() {
		logger := logger.With().
			Str("filename", fname).
			Logger()

		cfgErr := func(format string, args ...interface{}) error {
			path := filepath.Join(dir, fname)
			details := fmt.Sprintf(format, args...)
			return fmt.Errorf(
				"%w %s: %s",
				ErrMalformedTerramateConfig,
				path,
				details,
			)
		}

		// A cast error here would be a severe programming error on Terramate
		// side, so we are by design allowing the cast to panic
		body := hclfile.Body.(*hclsyntax.Body)

		logger.Trace().Msg("checking for attributes.")

		for name := range body.Attributes {
			return Config{}, cfgErr("unrecognized attribute %q", name)
		}

		var stackblock *hclsyntax.Block
		var tmblocks []*hclsyntax.Block
		var foundstack bool

		logger.Trace().Msg("Range over blocks.")

		for _, block := range body.Blocks {
			block.TypeRange.Filename = filepath.Join(dir, block.TypeRange.Filename)
			if !blockIsAllowed(block.Type) {
				return Config{}, cfgErr("block type %q is not supported", block.Type)
			}

			if block.Type == "terramate" {
				logger.Trace().Msg("Found 'terramate' block.")

				tmblocks = append(tmblocks, block)
				continue
			}

			if block.Type == "stack" {
				logger.Trace().Msg("Found stack block type.")

				if foundstack {
					return Config{}, cfgErr("duplicated stack block")
				}

				foundstack = true
				stackblock = block
			}
		}

		for _, tmblock := range tmblocks {
			if len(tmblock.Labels) > 0 {
				return Config{}, cfgErr("terramate block should not have labels")
			}

			if tmconfig.Terramate == nil {
				tmconfig.Terramate = &Terramate{}
			}

			tm := tmconfig.Terramate

			logger.Trace().Msg("Range over terramate block attributes.")

			for name, value := range tmblock.Body.Attributes {
				attrVal, diags := value.Expr.Value(nil)
				if diags.HasErrors() {
					return Config{}, cfgErr("evaluating %q: %w", name, diags)
				}
				switch name {
				case "required_version":
					logger.Trace().Msg("Parsing  attribute 'required_version'.")

					if attrVal.Type() != cty.String {
						return Config{}, cfgErr("attribute %q is not a string", name)
					}
					if tm.RequiredVersion != "" {
						return Config{}, cfgErr("attribute %q is duplicated", name)
					}
					tm.RequiredVersion = attrVal.AsString()

				default:
					return Config{}, cfgErr("unsupported attribute %q", name)
				}
			}

			logger.Trace().Msg("Range over terramate blocks")

			for _, block := range tmblock.Body.Blocks {
				switch block.Type {
				case "backend":
					logger.Trace().Msg("Parsing backend block.")

					if tm.Backend != nil {
						return Config{}, cfgErr("duplicated terramate.backend block")
					}

					if len(block.Labels) != 1 {
						return Config{}, cfgErr("backend block expects 1 label but has %v", block.Labels)
					}
					tm.Backend = block

				case "config":
					logger.Trace().Msg("Found config block.")

					if tm.RootConfig == nil {
						tm.RootConfig = &RootConfig{}
					}

					logger.Trace().Msg("Parse root config.")

					err := parseRootConfig(tm.RootConfig, block)
					if err != nil {
						return Config{}, cfgErr("parsing terramate.config: %v", err)
					}
				default:
					return Config{}, cfgErr("block type %q not supported", block.Type)
				}

			}
		}

		if !foundstack {
			continue
		}

		logger.Debug().Msg("Parsing stack cfg.")

		if tmconfig.Stack != nil {
			return Config{}, cfgErr("duplicated stack blocks across configs")
		}

		tmconfig.Stack = &Stack{}
		err := parseStack(tmconfig.Stack, stackblock)
		if err != nil {
			return Config{}, cfgErr("parsing stack: %v", err)
		}
	}

	return tmconfig, nil
}

type blockValidator func(*hclsyntax.Block) error

func parseHCLBlocks(dir, blocktype string, validate blockValidator) (HCLBlocks, error) {
	logger := log.With().
		Str("action", "hcl.parseHCLBlocks").
		Str("configdir", dir).
		Str("blocktype", blocktype).
		Logger()

	logger.Trace().Msg("loading config")

	parser, err := loadCfgBlocks(dir)
	if err != nil {
		return HCLBlocks{}, fmt.Errorf("parsing %q: %w", blocktype, err)
	}

	logger.Trace().Msg("Validating and filtering blocks")

	hclblocks := HCLBlocks{}

	for fname, hclfile := range parser.Files() {
		logger := logger.With().
			Str("filename", fname).
			Logger()

		logger.Trace().Msg("filtering blocks")
		// A cast error here would be a severe programming error on Terramate
		// side, so we are by design allowing the cast to panic
		body := hclfile.Body.(*hclsyntax.Body)
		blocks := filterBlocksByType(blocktype, body.Blocks)

		if len(blocks) == 0 {
			continue
		}

		logger.Trace().Msg("validating blocks")

		for _, block := range blocks {
			if err := validate(block); err != nil {
				return nil, fmt.Errorf("%q: %v", fname, err)
			}
		}

		logger.Trace().Msg("validated blocks")

		hclblocks[fname] = blocks
	}

	return hclblocks, nil
}

// IsLocal tells if module source is a local directory.
func (m Module) IsLocal() bool {
	// As specified here: https://www.terraform.io/docs/language/modules/sources.html#local-paths
	return m.Source[0:2] == "./" || m.Source[0:3] == "../"
}
