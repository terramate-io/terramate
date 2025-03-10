// Copyright 2025 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package commands

import "context"

type Executor interface {
	// Name of the comamnd.
	Name() string
	// Exec executes the command.
	Exec(ctx context.Context) error
}
