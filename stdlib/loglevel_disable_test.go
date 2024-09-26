// Copyright 2024 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package stdlib_test

import "github.com/rs/zerolog"

func init() {
	zerolog.SetGlobalLevel(zerolog.Disabled)
}
