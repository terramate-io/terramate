package main

import (
	"log"
	"os"

	"github.com/mineiros-io/terrastack/cmd/terrastack/cli"
)

func main() {
	wd, err := os.Getwd()
	if err != nil {
		log.Fatalf("failed to get process working dir: %v", err)
	}
	err = cli.Run(wd, os.Args[1:], os.Stdin, os.Stdout, os.Stderr)
	if err != nil {
		log.Fatal(err)
	}
}
