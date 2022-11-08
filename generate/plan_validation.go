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
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/mineiros-io/terramate/config"
	"github.com/mineiros-io/terramate/errors"
	"github.com/mineiros-io/terramate/hcl/eval"
	"github.com/mineiros-io/terramate/project"
	"github.com/rs/zerolog/log"
)

func (plan *Plan) addError(scope *config.Tree, errs ...error) {
	dir := scope.ProjDir()
	if _, ok := plan.Errors[dir]; !ok {
		plan.Errors[dir] = errors.L()
	}
	plan.Errors[dir].Append(errs...)
}

func (plan *Plan) AsError() error {
	errs := errors.L()
	for _, v := range plan.Errors {
		errs.Append(v)
	}
	return errs.AsError()
}

func (plan *Plan) validate() error {
	targetMap := make(map[project.Path]GenerateFile)
	for _, file := range plan.GenerateFiles {
		scope := file.Scope()
		if other, ok := targetMap[file.Target()]; ok {
			plan.addError(scope, errors.E(ErrConflictingConfig,
				"configs from %q and %q generate a file with same name %q have "+
					"`condition = true`",
				file.Origin().Path(),
				other.Origin().Path(),
				file.Target()))
		}
		targetMap[file.Target()] = file

		if file.Context() == RootContext {
			plan.addError(scope, plan.validateRootPath(file))
		} else {
			// context=stack

			plan.addError(scope,
				plan.validateStackPath(file),
				plan.checkAssertsFor(file),
			)
		}

	}
	plan.targetMap = targetMap

	return plan.AsError()
}

func (plan *Plan) validateRootPath(file GenerateFile) error {
	targetDir := file.Target().Dir()
	cfg, ok := plan.Scope.Lookup(targetDir)
	if ok && cfg.IsInsideStack() {
		return errors.E(
			ErrInvalidLabel,
			file.Origin(),
			"generate_*.context=root cannot generate inside stacks (%s is inside an stack)",
			targetDir,
		)
	}
	return nil
}

func (plan *Plan) validateStackPath(file GenerateFile) error {
	if !strings.HasPrefix(file.Target().String(), file.Scope().Dir()) {
		return errors.E(
			ErrInvalidLabel,
			"generate_*.context=stack requires a relative path leading to its scoped stack"+
				"but path is computed at %s (outside stack %s)",
			file.Target(),
			file.Scope().Dir(),
		)
	}
	stackpath := file.Scope().Dir()
	abspath := project.AbsPath(plan.Scope.RootDir(), file.Target().String())
	destdir := filepath.Dir(abspath)

	errs := errors.L()
	// We need to check that destdir, or any of its parents, is not a symlink or a stack.
	for strings.HasPrefix(destdir, stackpath) && destdir != stackpath {
		info, err := os.Lstat(destdir)
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				destdir = filepath.Dir(destdir)
				continue
			}
			errs.Append(errors.E(ErrInvalidLabel, err,
				file.Origin(),
				"%s: checking if dest dir is a symlink",
				file.Target()))
			break
		}
		if (info.Mode() & fs.ModeSymlink) == fs.ModeSymlink {
			errs.Append(errors.E(ErrInvalidLabel, err,
				file.Origin(),
				"%s: generates code inside a symlink",
				file.Target()))
			break
		}

		if config.IsStack(plan.Scope.Root(), destdir) {
			errs.Append(errors.E(ErrInvalidLabel,
				file.Origin(),
				"%s: generates code inside another stack %s",
				file.Target(),
				project.PrjAbsPath(plan.Scope.RootDir(), destdir)))
			break
		}
		destdir = filepath.Dir(destdir)
	}
	return nil
}

func (plan *Plan) checkAssertsFor(file GenerateFile) error {
	asserts, err := loadAsserts(file.Scope(), file.EvalContext())
	if err != nil {
		return err
	}

	asserts = append(asserts, file.Asserts()...)
	errs := errors.L()
	for _, assert := range asserts {
		if !assert.Assertion {
			if assert.Warning {
				log.Warn().
					Stringer("origin", assert.Range).
					Str("msg", assert.Message).
					Stringer("stack", file.Scope().ProjDir()).
					Msg("assertion failed")
			} else {
				msg := fmt.Sprintf("%s: %s", assert.Range, assert.Message)

				log.Debug().Msgf("assertion failure detected: %s", msg)

				err := errors.E(ErrAssertion, msg)
				errs.Append(err)
			}
		}
	}
	return errs.AsError()
}

func loadAsserts(tree *config.Tree, evalctx *eval.Context) ([]config.Assert, error) {
	asserts := []config.Assert{}
	assertConfigs := tree.UpwardAssertions()
	errs := errors.L()
	for _, assertCfg := range assertConfigs {
		assert, err := config.EvalAssert(evalctx, assertCfg)
		if err != nil {
			errs.Append(err)
		} else {
			asserts = append(asserts, assert)
		}
	}

	if err := errs.AsError(); err != nil {
		return nil, err
	}

	return asserts, nil
}
