// Copyright 2024 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

//go:build !profiler

package cli

func startProfiler(_ *CliSpec) {}
func stopProfiler(_ *CliSpec)  {}
