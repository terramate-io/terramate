// Copyright 2023 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

// Package trigger provides functionality that help manipulate stacks triggers.
package trigger

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
	"github.com/terramate-io/hcl/v2"
	"github.com/terramate-io/hcl/v2/hclparse"
	"github.com/terramate-io/hcl/v2/hclwrite"
	"github.com/terramate-io/terramate/config"
	"github.com/terramate-io/terramate/errors"
	"github.com/terramate-io/terramate/hcl/ast"
	"github.com/terramate-io/terramate/project"
	"github.com/zclconf/go-cty/cty"
)

const (
	// ErrTrigger indicates an error happened while triggering the stack.
	ErrTrigger errors.Kind = "trigger failed"

	// ErrParsing indicates an error happened while parsing the trigger file.
	ErrParsing errors.Kind = "parsing trigger file"
)

// Kind is the trigger type/kind.
type Kind string

// Supported trigger kinds.
const (
	Changed Kind = "changed"
	Ignored Kind = "ignore-change"
)

// Info represents the parsed contents of a trigger.
type Info struct {
	// Ctime is unix timestamp of when the trigger was created.
	Ctime int64
	// Reason is the reason why the trigger was created, if any.
	Reason string
	// Type is the trigger type.
	Type Kind
	// Context is the context of the trigger (only `stack` at the moment)
	Context string
	// StackPath is the path of the triggered stack.
	StackPath project.Path
}

const (
	// DefaultContext is the default context for the trigger file when not specified.
	DefaultContext = "stack"
)

const triggersDir = ".tmtriggers"

// StackPath accepts a trigger file path and returns the path of the stack
// that is triggered by the given file. If the given file is not a stack trigger
// at all it will return false.
func StackPath(triggerFile project.Path) (project.Path, bool) {
	const triggersPrefix = "/" + triggersDir

	if !triggerFile.HasPrefix(triggersPrefix) {
		return project.NewPath("/"), false
	}

	stackPath := strings.TrimPrefix(triggerFile.String(), triggersPrefix)
	stackPath = path.Dir(stackPath)
	
	// Ensure the stack path is absolute. path.Dir() can return "." for certain inputs
	// like empty strings, which would cause project.NewPath to panic.
	if stackPath == "." || stackPath == "" {
		stackPath = "/"
	} else if !path.IsAbs(stackPath) {
		stackPath = "/" + stackPath
	}
	
	return project.NewPath(stackPath), true
}

// Is checks if the filename is a trigger file and if that's the case then it returns
// the parsed trigger info and the triggered path.
// If the trigger file exists, the exists returns true.
// If the file is not inside the triggerDir then it returns an empty Info, empty path, false and no error.
// If there's an error parsing the trigger file, it returns empty info, true and the error.
// It only parses the file if it's inside the triggers directory (.tmtriggers).
func Is(root *config.Root, filename project.Path) (info Info, stack project.Path, exists bool, err error) {
	stackpath, exists := StackPath(filename)
	if !exists {
		return Info{}, stack, false, nil
	}
	info, err = ParseFile(filename.HostPath(root.HostDir()))
	if err != nil {
		return Info{}, stack, true, err
	}
	return info, stackpath, true, nil
}

// ParseFile will parse the given trigger file.
func ParseFile(path string) (Info, error) {
	parser := hclparse.NewParser()
	parsed, diags := parser.ParseHCLFile(path)
	if diags.HasErrors() {
		return Info{}, errors.E(ErrParsing, diags)
	}
	rootContent, diags := parsed.Body.Content(&hcl.BodySchema{
		Blocks: []hcl.BlockHeaderSchema{
			{
				Type: "trigger",
			},
		},
	})
	if diags.HasErrors() {
		return Info{}, errors.E(ErrParsing, diags, "checking trigger block schema")
	}

	if len(rootContent.Blocks) != 1 {
		return Info{}, errors.E(ErrParsing, "found %d blocks but expected 1")
	}

	triggerBlock := rootContent.Blocks[0]
	triggerContent, diags := triggerBlock.Body.Content(&hcl.BodySchema{
		Attributes: []hcl.AttributeSchema{
			{
				Name:     "ctime",
				Required: true,
			},
			{
				Name:     "reason",
				Required: true,
			},
			{
				Name:     "type",
				Required: false,
			},
			{
				Name:     "context",
				Required: false,
			},
		},
	})

	if diags.HasErrors() {
		return Info{}, errors.E(ErrParsing, diags, "checking trigger attributes schema")
	}

	errs := errors.L()
	info := Info{}

	for _, attribute := range ast.SortRawAttributes(triggerContent.Attributes) {
		if attribute.Name == "context" || attribute.Name == "type" {
			// they are keywords so they must be handled separately.
			keyword := hcl.ExprAsKeyword(attribute.Expr)

			switch attribute.Name {
			case "context":
				if keyword != DefaultContext {
					errs.Append(errors.E(
						"trigger: invalid trigger.context = %s (available options: %s)",
						keyword, DefaultContext,
					))
					continue
				}
				info.Context = keyword
			case "type":
				keyword := Kind(keyword)
				if keyword != Changed && keyword != Ignored {
					errs.Append(errors.E(
						"trigger: invalid trigger.type = %s (available options: %s, %s)",
						keyword, Changed, Ignored,
					))
					continue
				}
				info.Type = keyword
			}

			continue
		}

		val, err := attribute.Expr.Value(nil)
		if err != nil {
			errs.Append(errors.E(ErrParsing, "trigger: failure evaluating %q", attribute.Name))
			continue
		}

		switch attribute.Name {
		case "ctime":
			if val.Type() != cty.Number {
				errs.Append(errors.E(ErrParsing, "trigger: %s must be a number", attribute.Name))
				continue
			}
			v, _ := val.AsBigFloat().Int64()
			info.Ctime = v
		case "reason":
			if val.Type() != cty.String {
				errs.Append(errors.E(ErrParsing, "trigger: %s must be a string", attribute.Name))
				continue
			}
			info.Reason = val.AsString()
		default:
			errs.Append(errors.E(ErrParsing, "trigger: has unknown attribute %q", attribute.Name))
		}
	}

	if err := errs.AsError(); err != nil {
		return Info{}, err
	}

	// for backward compatibility (<= v0.2.7)
	if info.Type == "" {
		info.Type = Changed
	}

	if info.Context == "" {
		info.Context = DefaultContext
	}

	return info, nil
}

// Dir will return the triggers directory for the project rooted at rootdir.
// Both rootdir and the returned value are host absolute paths.
func Dir(rootdir string) string {
	return filepath.Join(rootdir, triggersDir)
}

func triggerFilename(kind Kind) (string, error) {
	id, err := uuid.NewRandom()
	if err != nil {
		return "", errors.E(err, "creating trigger UUID")
	}
	return fmt.Sprintf("%s-%s.tm.hcl", kind, id.String()), nil
}

// Create creates a trigger for a stack with the given path and the given reason
// inside the project rootdir.
func Create(root *config.Root, path project.Path, kind Kind, reason string) error {
	tree, ok := root.Lookup(path)
	if !ok || !tree.IsStack() {
		return errors.E(ErrTrigger, "path %s is not a stack directory", path)
	}
	filename, err := triggerFilename(kind)
	if err != nil {
		return errors.E(ErrTrigger, err)
	}
	triggerDir := filepath.Join(root.HostDir(), triggersDir, path.String())
	if err := os.MkdirAll(triggerDir, 0775); err != nil {
		return errors.E(ErrTrigger, err, "creating trigger dir")
	}

	ctime := time.Now().Unix()

	gen := hclwrite.NewEmptyFile()
	triggerBody := gen.Body().AppendNewBlock("trigger", nil).Body()
	triggerBody.SetAttributeValue("ctime", cty.NumberIntVal(ctime))
	triggerBody.SetAttributeValue("reason", cty.StringVal(reason))
	triggerBody.SetAttributeRaw("type", hclwrite.TokensForIdentifier(string(kind)))
	triggerBody.SetAttributeRaw("context", hclwrite.TokensForIdentifier(DefaultContext))

	triggerPath := filepath.Join(triggerDir, filename)

	if err := os.WriteFile(triggerPath, gen.Bytes(), 0666); err != nil {
		return errors.E(ErrTrigger, err, "creating trigger file")
	}

	log.Debug().
		Str("action", "trigger.Create").
		Int64("ctime", ctime).
		Str("reason", reason).
		Msg("trigger file created")

	return nil
}
