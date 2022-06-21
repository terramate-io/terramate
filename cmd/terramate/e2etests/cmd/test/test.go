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

// test is a test command that implements behaviors that are
// useful when testing terramate run features in a way that reduces
// dependencies on the environment to run the tests.
package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"
)

func main() {
	if len(os.Args) < 2 {
		log.Fatal("test requires at least one subcommand argument")
	}

	switch os.Args[1] {
	case "hang":
		hang()
	default:
		log.Fatalf("unknown command %s", os.Args[1])
	}
}

// hang will hang the process forever, ignoring any signals.
// It is useful to validate forced kill behavior.
// It will print the name of the received signals, which may also be useful in testing.
func hang() {
	signals := make(chan os.Signal, 1)
	signal.Notify(signals)

	for s := range signals {
		fmt.Println(s)
	}
}
