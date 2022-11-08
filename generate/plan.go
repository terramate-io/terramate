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

package generate

import (
	"io/fs"
	"os"

	"github.com/mineiros-io/terramate/config"
	"github.com/mineiros-io/terramate/errors"
	"github.com/mineiros-io/terramate/hcl/eval"
	"github.com/mineiros-io/terramate/hcl/info"
	"github.com/mineiros-io/terramate/project"
	"github.com/mineiros-io/terramate/stack"
)

const (
	StackContext = "stack"
	RootContext  = "root"
)

const (
	// ErrConflictingConfig indicates that two code generation configurations
	// are conflicting, like both generates a file with the same name and would
	// overwrite each other.
	ErrConflictingConfig errors.Kind = "conflicting config detected"

	// ErrInvalidRootLabel indicates that a generate block with context=root
	// has an invalid label.
	ErrInvalidLabel errors.Kind = "invalid label in generate block"

	// ErrAssertion indicates that code generation configuration has a failed
	// assertion.
	ErrAssertion errors.Kind = "assertion failed"

	// ErrManualCodeExists indicates code generation would replace code that
	// was not previously generated by Terramate.
	ErrManualCodeExists errors.Kind = "manually defined code found"
)

type GenerateFile interface {
	Target() project.Path
	Header() string
	Condition() bool
	Context() string
	Content() string
	Asserts() []config.Assert

	Scope() *config.Tree
	EvalContext() *eval.Context
	Origin() info.Range
}

type GenerateFiles []GenerateFile

type Plan struct {
	Scope         *config.Tree
	GenerateFiles []GenerateFile
	Create        []project.Path
	Change        []project.Path
	Delete        []project.Path
	Errors        map[project.Path]*errors.List

	stacks   stack.List
	projmeta project.Metadata

	targetMap map[project.Path]GenerateFile
	createMap map[project.Path]GenerateFile
	changeMap map[project.Path]GenerateFile
	deleteMap map[project.Path]GenerateFile
}

// NewPlan creates a new generate plan.
func NewPlan(scope *config.Tree) (*Plan, error) {
	stackNodes := scope.Stacks()
	stacks := make(stack.List, len(stackNodes))
	stackPaths := make(project.Paths, len(stackNodes))
	for i, node := range stackNodes {
		st, err := stack.New(node)
		if err != nil {
			return nil, err
		}
		stacks[i] = st
		stackPaths[i] = st.Path()
	}

	return &Plan{
		Scope:  scope,
		Errors: make(map[project.Path]*errors.List),

		stacks:    stacks,
		projmeta:  project.NewMetadata(scope.RootDir(), stackPaths),
		createMap: make(map[project.Path]GenerateFile),
		changeMap: make(map[project.Path]GenerateFile),
		deleteMap: make(map[project.Path]GenerateFile),
	}, nil
}

func DoPlan(scope *config.Tree) (*Plan, error) {
	plan, err := NewPlan(scope)
	if err != nil {
		return nil, err
	}
	genfiles, err := plan.selection()
	if err != nil {
		return nil, err
	}
	genfiles = genfiles.FilterBy(func(f GenerateFile) bool {
		return f.Condition()
	})
	plan.GenerateFiles = genfiles
	err = plan.validate()
	if err != nil {
		return nil, err
	}
	plan.plan()
	return plan, nil
}

// selection implements generation phase 1.
// It selects all relevant generate blocks based on current scope configuration.
// For stacks, it filters all generate blocks of context=stack in current scope
// node and up in the tree until root.
// For root, it filters all generate blocks of context=root in current scope
// node and down in the tree reaching all the leaves.
func (plan *Plan) selection() (GenerateFiles, error) {
	stackFiles, err := plan.selectStackGenerateFiles()
	if err != nil {
		return nil, errors.E(err, "selecting generate blocks for stack scope")
	}

	rootFiles, err := plan.selectRootGenerateFiles()
	if err != nil {
		return nil, errors.E(err, "selecting generate blocks for root scope")
	}

	genfiles := append(GenerateFiles{}, stackFiles...)
	genfiles = append(genfiles, rootFiles...)
	return genfiles, nil
}

func (plan *Plan) plan() {
	for _, file := range plan.GenerateFiles {
		plan.addError(file.Scope(), plan.planFile(file))
	}
	return
}

func (plan *Plan) planFile(file GenerateFile) error {
	abspath := project.AbsPath(plan.Scope.RootDir(), file.Target().String())
	st, err := os.Lstat(abspath)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			plan.createMap[file.Target()] = file
			return nil
		}
		return err
	}
	if st.IsDir() {
		return errors.E(ErrInvalidLabel, "generate label is an existent directory")
	}
	if !st.Mode().IsRegular() {
		return errors.E(ErrInvalidLabel,
			"generate label points to a file that's not regular: %s",
			st.Mode(),
		)
	}
	gotContent, err := os.ReadFile(abspath)
	if !isEqualContent(file, string(gotContent)) {
		if file.Header() == "" {
			return errors.E(ErrManualCodeExists, "check file %q", abspath)
		}
		plan.changeMap[file.Target()] = file
		return nil
	}
	return nil
}

func isEqualContent(file GenerateFile, content string) bool {
	generated := file.Header()
	if generated != "" {
		generated += "\n"
	}
	generated += file.Content()
	return generated == content
}

func (plan *Plan) selectStackGenerateFiles() (GenerateFiles, error) {
	var genfiles GenerateFiles
	for _, st := range plan.stacks {
		_, globalsReport := stack.LoadStackGlobals(plan.Scope, plan.projmeta, st)
		if err := globalsReport.AsError(); err != nil {
			return nil, err
		}

		scope, _ := plan.Scope.Root().Lookup(st.Path())

		evalctx := stack.NewEvalCtx(plan.Scope, plan.projmeta, st, globalsReport.Globals)
		blocks := plan.Scope.UpwardGenerateBlocks()
		for _, block := range blocks {
			// generate_file
			for _, fileBlock := range block.Files {
				if fileBlock.Context != StackContext {
					continue
				}
				file, err := config.EvalGenerate(evalctx.Context, fileBlock, scope)
				if err != nil {
					return nil, err
				}

				genfiles = append(genfiles, file)
			}

			// generate_hcl
			for _, hclBlock := range block.HCLs {
				if hclBlock.Context != StackContext {
					continue
				}
				file, err := config.EvalGenerate(evalctx.Context, hclBlock, scope)
				if err != nil {
					return nil, err
				}

				genfiles = append(genfiles, file)
			}
		}
	}
	return genfiles, nil
}

func (plan *Plan) selectRootGenerateFiles() (GenerateFiles, error) {
	var genfiles GenerateFiles

	blocks := plan.Scope.DownwardGenerateBlocks()
	evalctx, err := eval.NewContext(plan.Scope.RootDir(), plan.Scope.ProjDir())
	if err != nil {
		return nil, err
	}

	evalctx.SetNamespace("terramate", plan.projmeta.ToCtyMap())
	for _, block := range blocks {
		// generate_file
		for _, fileBlock := range block.Files {
			if fileBlock.Context != RootContext {
				continue
			}
			scope, _ := plan.Scope.Root().Lookup(fileBlock.Range.Path())
			file, err := config.EvalGenerate(evalctx, fileBlock, scope)
			if err != nil {
				return nil, err
			}

			genfiles = append(genfiles, file)
		}

		// TODO(i4k): implement generate_hcl for root context (when needed)
	}
	return genfiles, nil
}

func (files GenerateFiles) FilterBy(filter func(file GenerateFile) bool) GenerateFiles {
	var result GenerateFiles
	for _, f := range files {
		if filter(f) {
			result = append(result, f)
		}
	}
	return result
}
