// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

package globals

import (
	"github.com/terramate-io/terramate/config"
	"github.com/terramate-io/terramate/hcl/eval"
	"github.com/terramate-io/terramate/stdlib"
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
