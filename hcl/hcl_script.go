// Copyright 2023 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package hcl

import (
	"github.com/terramate-io/terramate/errors"
	"github.com/terramate-io/terramate/hcl/ast"
)

// Errors returned during the HCL parsing of script block
const (
	ErrScriptNoLabels            errors.Kind = "terramate schema error: (script): must provide at least one label"
	ErrScriptNoDesc              errors.Kind = "terramate schema error: (script): missing description"
	ErrScriptUnrecognizedAttr    errors.Kind = "terramate schema error: (script): unrecognized attribute"
	ErrScriptUnrecognizedBlock   errors.Kind = "terramate schema error: (script): unrecognized block"
	ErrScriptNoCmds              errors.Kind = "terramate schema error: (script): missing command or commands"
	ErrScriptMissingOrInvalidJob errors.Kind = "terramate schema error: (script): missing or invalid job"
	ErrScriptCmdConflict         errors.Kind = "terramate schema error: (script): conflicting attribute already set"
)

// Command represents an executable command
type Command ast.Attribute

// Commands represents a list of executable commands
type Commands ast.Attribute

// ScriptJob represent a Job within a Script
type ScriptJob struct {
	Command  *Command  // Command is a single executable command
	Commands *Commands // Commands is a list of executable commands
}

// ScriptDescription is human readable description of a script
type ScriptDescription ast.Attribute

// Script represents a parsed script block
type Script struct {
	Labels      []string           // Labels of the script block used for grouping scripts
	Description *ScriptDescription // Description is a human readable description of a script
	Jobs        []*ScriptJob       // Job represents the command(s) part of this script
}

// NewScriptDescription returns a *ScriptDescription encapsulating an ast.Attribute
func NewScriptDescription(attr ast.Attribute) *ScriptDescription {
	desc := ScriptDescription(attr)
	return &desc
}

// NewScriptCommand returns a *Command encapsulating an ast.Attribute
func NewScriptCommand(attr ast.Attribute) *Command {
	cmd := Command(attr)
	return &cmd
}

// NewScriptCommands returns *Commands encapsulating an ast.Attribute
func NewScriptCommands(attr ast.Attribute) *Commands {
	cmds := Commands(attr)
	return &cmds
}

func (p *TerramateParser) parseScriptBlock(block *ast.Block) (*Script, error) {
	errs := errors.L()

	parsedScript := &Script{
		Labels: block.Labels,
	}

	for _, attr := range block.Attributes {
		switch attr.Name {
		case "description":
			parsedScript.Description = NewScriptDescription(attr)
		default:
			errs.Append(errors.E(ErrScriptUnrecognizedAttr, attr.NameRange))
		}
	}

	for _, nestedBlock := range block.Blocks {
		switch nestedBlock.Type {
		case "job":
			parsedJobBlock, err := validateScriptJobBlock(nestedBlock)
			if err != nil {
				errs.Append(err)
				continue
			}
			parsedScript.Jobs = append(parsedScript.Jobs, parsedJobBlock)
		default:
			errs.Append(errors.E(ErrScriptUnrecognizedBlock, nestedBlock.TypeRange, nestedBlock.Type))

		}
	}

	if len(parsedScript.Labels) == 0 {
		errs.Append(errors.E(ErrScriptNoLabels, block.TypeRange))
	}

	if len(parsedScript.Jobs) == 0 {
		errs.Append(errors.E(ErrScriptMissingOrInvalidJob, block.OpenBraceRange))
	}

	if parsedScript.Description == nil {
		errs.Append(errors.E(ErrScriptNoDesc, block.OpenBraceRange))
	}

	if err := errs.AsError(); err != nil {
		return nil, err
	}

	return parsedScript, nil

}

func validateScriptJobBlock(block *ast.Block) (*ScriptJob, error) {
	errs := errors.L()

	parsedScriptJob := &ScriptJob{}
	for _, attr := range block.Attributes {
		switch attr.Name {
		case "command":
			parsedScriptJob.Command = NewScriptCommand(attr)
		case "commands":
			parsedScriptJob.Commands = NewScriptCommands(attr)
		default:
			errs.Append(errors.E(ErrScriptUnrecognizedAttr, attr.NameRange, attr.Name))

		}
	}

	for _, childBlock := range block.Blocks {
		errs.Append(errors.E(ErrScriptUnrecognizedBlock, childBlock.TypeRange, childBlock.Type))
	}

	// job.command and job.commands are mutually exclusive
	if parsedScriptJob.Command != nil && parsedScriptJob.Commands != nil {
		errs.Append(errors.E(ErrScriptCmdConflict, parsedScriptJob.Command.NameRange))
		errs.Append(errors.E(ErrScriptCmdConflict, parsedScriptJob.Commands.NameRange))
	}

	if parsedScriptJob.Command == nil && parsedScriptJob.Commands == nil {
		errs.Append(errors.E(ErrScriptNoCmds, block.TypeRange))
	}

	if err := errs.AsError(); err != nil {
		return nil, err
	}

	return parsedScriptJob, nil
}
