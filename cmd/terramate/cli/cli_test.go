package cli_test

import (
	"bytes"
	"testing"

	"github.com/terramate-io/terramate/cmd/terramate/cli"
)

type testCommand struct {
	Flag int `help:"A flag."`
}

const cmdName = "my-test-command"

func TestCLICommandExtension(t *testing.T) {
	t.Parallel()

	var stdin bytes.Buffer
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	tcmd := testCommand{}

	ran := false
	run := func(cli *cli.CLI, cmd string) bool {
		if cmd != cmdName {
			t.Errorf("unexpected command: %s", cmd)
		}
		ran = true
		return true
	}

	cli.Exec("0.0.1", []string{cmdName}, &stdin, &stdout, &stderr, &cli.ExtraCommandHandler{
		Commands: cli.ExtraCommands{
			cli.ExtraCommand{
				Name: cmdName,
				Help: "A test command",
				Spec: &tcmd,
			},
		},
		RunAfterConfig: run,
	})
	if !ran {
		t.Fatalf("command not run")
	}
}
