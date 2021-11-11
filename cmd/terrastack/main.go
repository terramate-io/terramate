package main

import (
	"log"
	"os"

	"github.com/mineiros-io/terrastack/cmd/terrastack/cli"
)

func main() {
	basedir, err := os.Getwd()
	if err != nil {
		log.Fatalf("error: failed to get current directory: %v", err)
	}
	if err := cli.Run(os.Args, basedir, os.Stdin, os.Stdout, os.Stderr); err != nil {
		log.Fatal(err)
	}
}
