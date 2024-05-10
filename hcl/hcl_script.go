// Copyright 2023 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package hcl

import (
	"strings"

	"github.com/terramate-io/terramate/errors"
	"github.com/terramate-io/terramate/hcl/ast"
	"github.com/terramate-io/terramate/hcl/info"
	"golang.org/x/exp/slices"
)

// Errors returned during the HCL parsing of script block
const (
	ErrScriptNoLabels            errors.Kind = "terramate schema error: (script): must provide at least one label"
	ErrScriptRedeclared          errors.Kind = "terramate schema error: (script): multiple script blocks with same labels in the same directory"
	ErrScriptUnrecognizedAttr    errors.Kind = "terramate schema error: (script): unrecognized attribute"
	ErrScriptJobUnrecognizedAttr errors.Kind = "terramate schema error: (script.job): unrecognized attribute"
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
	Name        *ast.Attribute
	Description *ast.Attribute
	Command     *Command  // Command is a single executable command
	Commands    *Commands // Commands is a list of executable commands
}

// Script represents a parsed script block
type Script struct {
	Range       info.Range
	Labels      []string       // Labels of the script block used for grouping scripts
	Name        *ast.Attribute // Name of the script
	Description *ast.Attribute // Description is a human readable description of a script
	Jobs        []*ScriptJob   // Job represents the command(s) part of this script
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

// AccessorName returns the name traversal for accessing the script.
func (sc *Script) AccessorName() string {
	var b strings.Builder
	for i, e := range sc.Labels {
		if i != 0 {
			_ = b.WriteByte(' ')
		}
		if strings.Contains(e, " ") {
			_ = b.WriteByte('"')
			_, _ = b.WriteString(e)
			_ = b.WriteByte('"')
		} else {
			_, _ = b.WriteString(e)
		}
	}
	return b.String()
}

func (p *TerramateParser) parseScriptBlock(block *ast.Block) (*Script, error) {
	errs := errors.L()

	parsedScript := &Script{
		Range:  block.Range,
		Labels: block.Labels,
	}

	for _, attr := range block.Attributes {
		attr := attr
		switch attr.Name {
		case "name":
			parsedScript.Name = &attr
		case "description":
			parsedScript.Description = &attr
		default:
			errs.Append(errors.E(ErrScriptUnrecognizedAttr, attr.NameRange))
		}
	}

	for _, nestedBlock := range block.Blocks {
		switch nestedBlock.Type {
		case "job":
			parsedJobBlock, err := parseScriptJobBlock(nestedBlock)
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
		errs.Append(errors.E(ErrScriptMissingOrInvalidJob, block.Range))
	}

	if err := errs.AsError(); err != nil {
		return nil, err
	}

	return parsedScript, nil
}

func findScript(scripts []*Script, target []string) (*Script, bool) {
	for _, script := range scripts {
		if slices.Equal(script.Labels, target) {
			return script, true
		}
	}
	return nil, false
}

func parseScriptJobBlock(block *ast.Block) (*ScriptJob, error) {
	errs := errors.L()

	parsedScriptJob := &ScriptJob{}
	for _, attr := range block.Attributes {
		attr := attr
		switch attr.Name {
		case "name":
			parsedScriptJob.Name = &attr
		case "description":
			parsedScriptJob.Description = &attr
		case "command":
			parsedScriptJob.Command = NewScriptCommand(attr)
		case "commands":
			parsedScriptJob.Commands = NewScriptCommands(attr)
		default:
			errs.Append(errors.E(ErrScriptJobUnrecognizedAttr, attr.NameRange, attr.Name))
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
		errs.Append(errors.E(ErrScriptNoCmds, block.Range))
	}

	if err := errs.AsError(); err != nil {
		return nil, err
	}

	return parsedScriptJob, nil
}
