// Copyright 2022 Mineiros GmbH
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
	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclparse"
	"github.com/hashicorp/hcl/v2/hclwrite"
	"github.com/mineiros-io/terramate/config"
	"github.com/mineiros-io/terramate/errors"
	"github.com/mineiros-io/terramate/hcl/ast"
	"github.com/mineiros-io/terramate/project"
	"github.com/rs/zerolog/log"
	"github.com/zclconf/go-cty/cty"
)

const (
	// ErrTrigger indicates an error happened while triggering the stack.
	ErrTrigger errors.Kind = "trigger failed"

	// ErrParsing indicates an error happened while parsing the trigger file.
	ErrParsing errors.Kind = "parsing trigger file"
)

// Info represents the parsed contents of a trigger
// for triggers created by Terramate.
type Info struct {
	// Ctime is unix timestamp of when the trigger was created.
	Ctime int64
	// Reason is the reason why the trigger was created, if any.
	Reason string
}

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
	return project.NewPath(stackPath), true
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
		},
	})

	if diags.HasErrors() {
		return Info{}, errors.E(ErrParsing, diags, "checking trigger attributes schema")
	}

	errs := errors.L()
	info := Info{}
	for _, attribute := range ast.SortRawAttributes(triggerContent.Attributes) {
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
	return info, nil
}

// Dir will return the triggers directory for the project rooted at rootdir.
// Both rootdir and the returned value are host absolute paths.
func Dir(rootdir string) string {
	return filepath.Join(rootdir, triggersDir)
}

func triggerFilename() (string, error) {
	id, err := uuid.NewRandom()
	if err != nil {
		return "", errors.E(err, "creating trigger UUID")
	}
	return fmt.Sprintf("changed-%s.tm.hcl", id.String()), nil
}

// Create creates a trigger for a stack with the given path and the given reason
// inside the project rootdir.
func Create(root *config.Root, path project.Path, reason string) error {
	tree, ok := root.Lookup(path)
	if !ok || !tree.IsStack() {
		return errors.E(ErrTrigger, "path %s is not a stack directory", path)
	}
	filename, err := triggerFilename()
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
