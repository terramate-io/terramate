package main

import (
	"log"
	"os"

	"github.com/mineiros-io/terrastack/cmd/terrastack/cli"
)

func main() {
	if err := cli.Run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr); err != nil {
		log.Fatal(err)
	}
}
