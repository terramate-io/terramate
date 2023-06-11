// Copyright 2023 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package e2etest

import "github.com/rs/zerolog"

func init() {
	zerolog.SetGlobalLevel(zerolog.Disabled)
}
