// Copyright 2025 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

// Package commands define all Terramate commands.
// All commands must:
// - Be defined in its own package (eg.: ./commands/generate)
// - Define a `<cmd>.Spec` type declaring all variables that control the command behavior.
// - Implement the [commands.Executor] interface.
// - Never abort, panic or exit the process.
package commands
