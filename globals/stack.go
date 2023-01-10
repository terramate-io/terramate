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

package globals

import (
	"github.com/mineiros-io/terramate/config"
	"github.com/mineiros-io/terramate/hcl/eval"
	"github.com/mineiros-io/terramate/stdlib"
)

// ForStack loads from the config tree all globals defined for a given stack.
func ForStack(root *config.Root, stack *config.Stack) EvalReport {
	ctx := eval.NewContext(
		stdlib.Functions(stack.HostDir(root)),
	)
	runtime := root.Runtime()
	runtime.Merge(stack.RuntimeValues(root))
	ctx.SetNamespace("terramate", runtime)
	return ForDir(root, stack.Dir, ctx)
}
