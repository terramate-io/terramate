// Copyright 2023 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package resource

import "context"

// R is the resource interface.
type R interface {
	Acquire(ctx context.Context) bool
	Release()
}
