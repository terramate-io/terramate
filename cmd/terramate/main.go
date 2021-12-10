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

package main

import (
	"log"
	"os"
	"runtime/debug"

	"github.com/mineiros-io/terramate"
	"github.com/mineiros-io/terramate/cmd/terramate/cli"
	"golang.org/x/mod/semver"
)

func main() {
	wd, err := os.Getwd()
	if err != nil {
		log.Fatalf("failed to get process working dir: %v", err)
	}
	defineVersion()
	err = cli.Run(wd, os.Args[1:], os.Stdin, os.Stdout, os.Stderr)
	if err != nil {
		log.Fatal(err)
	}
}

func defineVersion() {
	const (
		defaultVersion = "0.0.0-devel"
	)

	if terramate.Version != "" {
		return
	}

	info, available := debug.ReadBuildInfo()
	if !available {
		terramate.Version = defaultVersion
		return
	}

	// Difference scenarios have all kind of build info version
	// we only want valid semver, or else explosions !!
	if !semver.IsValid(info.Main.Version) {
		terramate.Version = defaultVersion
		return
	}

	// Go adds v prefix to the semver...
	terramate.Version = info.Main.Version[1:]
}
