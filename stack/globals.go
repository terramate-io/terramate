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

package stack

import (
	"github.com/mineiros-io/terramate/config"
	"github.com/mineiros-io/terramate/errors"
	"github.com/mineiros-io/terramate/globals"
	"github.com/mineiros-io/terramate/hcl/eval"
	"github.com/mineiros-io/terramate/project"
	"github.com/mineiros-io/terramate/stdlib"
	"github.com/rs/zerolog/log"
)

// LoadStackGlobals loads from the config tree all globals defined for a given
// stack. It will navigate the configuration tree from the stack dir until
// it reaches root, loading globals and merging them appropriately.
//
// More specific globals (closer or at the stack) have precedence over
// less specific globals (closer or at the root dir).
//
// Metadata for the stack is used on the evaluation of globals.
// The rootdir MUST be an absolute path.
func LoadStackGlobals(root *config.Root, projmeta project.Metadata, stackmeta Metadata) globals.EvalReport {
	logger := log.With().
		Str("action", "stack.LoadStackGlobals()").
		Stringer("stack", stackmeta.Path()).
		Logger()

	logger.Debug().Msg("Creating stack context.")

	funcs, err := stdlib.NewFunctions(stackmeta.HostPath())
	if err != nil {
		panic(errors.E(errors.ErrInternal, "failed to instantiate stdlib"))
	}
	ctx := eval.NewContext(funcs)
	ctx.SetNamespace("terramate", MetadataToCtyValues(projmeta, stackmeta))
	return globals.Load(root, stackmeta.Path(), ctx)
}
