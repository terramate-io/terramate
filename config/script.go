// Copyright 2023 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package config

import (
	hhcl "github.com/hashicorp/hcl/v2"
	"github.com/terramate-io/terramate/errors"
	"github.com/terramate-io/terramate/hcl"
	"github.com/terramate-io/terramate/hcl/eval"
	"github.com/terramate-io/terramate/hcl/info"
	"github.com/zclconf/go-cty/cty"
)

// Errors for indicating invalid script schema
const (
	ErrScriptSchema              errors.Kind = "script config has an invalid schema"
	ErrScriptInvalidTypeDesc     errors.Kind = "invalid type for script.description"
	ErrScriptInvalidTypeCommand  errors.Kind = "invalid type for script.command"
	ErrScriptInvalidTypeCommands errors.Kind = "invalid type for script.commands"
	ErrScriptEmptyCmds           errors.Kind = "job command or commands evaluated to empty list"
)

// ScriptJob represents an evaluated job block
type ScriptJob struct {
	Cmd  []string
	Cmds [][]string
}

// Script represents an evaluated script block
type Script struct {
	Range       info.Range
	Labels      []string
	Description string
	Jobs        []ScriptJob
}

// Commands is a convenience method for callers who don't specifically
// care about which command attr was set e.g. command or commands. This method
// returns a list of commands irrespective of whether they were set through
// job.command or job.commands
func (es ScriptJob) Commands() [][]string {
	if es.Cmd != nil {
		return [][]string{es.Cmd}
	}
	return es.Cmds
}

// EvalScript evaluates a script block using the provided evaluation context
func EvalScript(evalctx *eval.Context, script hcl.Script) (Script, error) {
	evaluatedScript := Script{
		Range:  script.Range,
		Labels: script.Labels,
	}
	errs := errors.L()

	desc, err := evalScriptDesc(evalctx, script.Description.Expr, "script.description")
	errs.Append(err)

	evaluatedScript.Description = desc

	for _, job := range script.Jobs {
		evaluatedJob := ScriptJob{}

		if job.Command != nil {
			command, err := evalScriptJobCommand(evalctx, job.Command.Expr, "command")
			if err != nil {
				errs.Append(err)
				continue
			}
			evaluatedJob.Cmd = command
		}

		if job.Commands != nil {
			commands, err := evalScriptJobCommands(evalctx, job.Commands.Expr, "commands")
			if err != nil {
				errs.Append(err)
				continue
			}
			evaluatedJob.Cmds = commands
		}

		evaluatedScript.Jobs = append(evaluatedScript.Jobs, evaluatedJob)
	}

	if err := errs.AsError(); err != nil {
		return Script{}, err
	}

	return evaluatedScript, nil
}

func evalScriptDesc(evalctx *eval.Context, expr hhcl.Expression, name string) (string, error) {
	desc, err := evalString(evalctx, expr, name)
	if err != nil {
		return "", errors.E(ErrScriptInvalidTypeDesc, expr.Range(), err)
	}

	return desc, nil
}

func evalScriptJobCommand(evalctx *eval.Context, expr hhcl.Expression, name string) ([]string, error) {
	val, err := evalctx.Eval(expr)
	if err != nil {
		return nil, errors.E(ErrScriptSchema, expr.Range(),
			err, "evaluating %s", name)
	}

	if !val.Type().IsTupleType() {
		return nil, errors.E(ErrScriptInvalidTypeCommand, expr.Range(),
			"%s should be a list(string) type", name)
	}

	if val.LengthInt() == 0 {
		return nil, errors.E(ErrScriptEmptyCmds, expr.Range())
	}

	errs := errors.L()
	evaluatedCommand := []string{}

	index := -1
	it := val.ElementIterator()
	for it.Next() {
		index++
		_, elem := it.Element()
		if elem.Type() != cty.String {
			errs.Append(errors.E(ErrScriptInvalidTypeCommand, expr.Range(),
				"field %s must be a list(string) but element %d has type %q",
				name, index, elem.Type().FriendlyName()))
			continue
		}
		evaluatedCommand = append(evaluatedCommand, elem.AsString())
	}

	if err := errs.AsError(); err != nil {
		return nil, err
	}

	return evaluatedCommand, nil
}

func evalScriptJobCommands(evalctx *eval.Context, expr hhcl.Expression, name string) ([][]string, error) {
	val, err := evalctx.Eval(expr)
	if err != nil {
		return nil, errors.E(ErrScriptSchema, expr.Range(),
			err, "evaluating %s", name)
	}

	if !val.Type().IsTupleType() {
		return nil, errors.E(ErrScriptInvalidTypeCommands, expr.Range(),
			"%s should be a list(string) type", name)
	}

	if val.LengthInt() == 0 {
		return nil, errors.E(ErrScriptEmptyCmds, expr.Range())
	}

	errs := errors.L()
	evaluatedCommands := [][]string{}

	index := -1
	it := val.ElementIterator()
	for it.Next() {
		index++
		_, elem := it.Element()
		if !elem.Type().IsTupleType() {
			errs.Append(errors.E(ErrScriptInvalidTypeCommands, expr.Range(),
				"field %s must be a list of list(string) but element %d has type %q",
				name, index, elem.Type().FriendlyName()))
			continue
		}

		if elem.LengthInt() == 0 {
			return nil, errors.E(ErrScriptEmptyCmds, expr.Range(), "commands item %d is empty", index)
		}

		evaluatedCommand := []string{}
		indexCommand := -1
		itCommand := elem.ElementIterator()
		for itCommand.Next() {
			indexCommand++
			_, elem := itCommand.Element()
			if elem.Type() != cty.String {
				errs.Append(errors.E(ErrScriptInvalidTypeCommands, expr.Range(),
					"field %s must be a string but element %d has type %q",
					name, indexCommand, elem.Type().FriendlyName()))
				continue
			}
			evaluatedCommand = append(evaluatedCommand, elem.AsString())
		}

		evaluatedCommands = append(evaluatedCommands, evaluatedCommand)
	}

	if err := errs.AsError(); err != nil {
		return nil, err
	}

	return evaluatedCommands, nil
}
