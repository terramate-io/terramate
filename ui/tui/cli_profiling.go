// Copyright 2024 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

//go:build profiler

package tui

import (
	stdfmt "fmt"
	"os"
	"runtime/pprof"

	"github.com/terramate-io/terramate/printer"
)

func startProfiler(enable bool) {
	if !enable {
		return
	}
	const defaultProfilerName = "terramate.prof"

	fname := os.Getenv("TM_TEST_PROFILING_PATH")
	if fname == "" {
		fname = defaultProfilerName
	}

	stdfmt.Printf("Creating CPU profile (%s)...\n", fname)

	f, err := os.Create(fname)
	if err != nil {
		printer.Stderr.Fatal(err)
	}
	err = pprof.StartCPUProfile(f)
	if err != nil {
		printer.Stderr.Fatal(err)
	}
}

func stopProfiler(enable bool) {
	if !enable {
		return
	}
	pprof.StopCPUProfile()
}
