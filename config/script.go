// Copyright 2023 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package config

import (
	"fmt"
	"strings"

	hhcl "github.com/hashicorp/hcl/v2"
	"github.com/terramate-io/terramate/cloud/preview"
	"github.com/terramate-io/terramate/errors"
	"github.com/terramate-io/terramate/hcl"
	"github.com/terramate-io/terramate/hcl/eval"
	"github.com/terramate-io/terramate/hcl/info"
	"github.com/terramate-io/terramate/printer"
	"github.com/zclconf/go-cty/cty"
)

// Errors for indicating invalid script schema
const (
	ErrScriptSchema              errors.Kind = "script config has an invalid schema"
	ErrScriptInvalidType         errors.Kind = "invalid type for script field"
	ErrScriptInvalidTypeCommand  errors.Kind = "invalid type for script.command"
	ErrScriptInvalidTypeCommands errors.Kind = "invalid type for script.commands"
	ErrScriptEmptyCmds           errors.Kind = "job command or commands evaluated to empty list"
	ErrScriptInvalidCmdOptions   errors.Kind = "invalid options for script command"
)

// MaxScriptNameRunes defines the maximum number of runes allowed for a script name.
const MaxScriptNameRunes = 128

// MaxScriptDescRunes defines the maximum number of runes allowed for a script description.
const MaxScriptDescRunes = 1000

// ScriptCmdOptions represents optional parameters for a script command
type ScriptCmdOptions struct {
	CloudSyncDeployment    bool
	CloudSyncDriftStatus   bool
	CloudSyncPreview       bool
	CloudSyncLayer         preview.Layer
	CloudSyncTerraformPlan string
	UseTerragrunt          bool
}

// ScriptCmd represents an evaluated script command
type ScriptCmd struct {
	Args    []string
	Options *ScriptCmdOptions
}

// ScriptJob represents an evaluated job block
type ScriptJob struct {
	Name        string
	Description string
	Cmd         *ScriptCmd
	Cmds        []*ScriptCmd
}

// Script represents an evaluated script block
type Script struct {
	Range       info.Range
	Labels      []string
	Name        string
	Description string
	Jobs        []ScriptJob
}

// Commands is a convenience method for callers who don't specifically
// care about which command attr was set e.g. command or commands. This method
// returns a list of commands irrespective of whether they were set through
// job.command or job.commands
func (es ScriptJob) Commands() []*ScriptCmd {
	if es.Cmd != nil {
		return []*ScriptCmd{es.Cmd}
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
	if script.Name != nil {
		name, err := evalScriptStringField(evalctx, script.Name.Expr, "script.name")
		errs.Append(err)
		if len(name) > MaxScriptNameRunes {
			name = name[:MaxScriptNameRunes]

			printer.Stderr.Warn(
				fmt.Sprintf("`script.name` exceeds the maximum allowed characters (%d): field truncated", MaxScriptNameRunes),
			)
		}
		evaluatedScript.Name = name
	}

	if script.Description != nil {
		desc, err := evalScriptStringField(evalctx, script.Description.Expr, "script.description")
		errs.Append(err)
		if len(desc) > MaxScriptDescRunes {
			desc = desc[:MaxScriptDescRunes]

			printer.Stderr.Warn(
				fmt.Sprintf("`script.description` exceeds the maximum allowed characters (%d): field truncated", MaxScriptDescRunes),
			)
		}
		evaluatedScript.Description = desc
	}

	for _, job := range script.Jobs {
		evaluatedJob := ScriptJob{}

		if job.Name != nil {
			name, err := evalScriptStringField(evalctx, job.Name.Expr, "script.job.name")
			errs.Append(err)
			if len(name) > MaxScriptNameRunes {
				name = name[:MaxScriptNameRunes]

				printer.Stderr.Warn(
					fmt.Sprintf("`script.job.name` exceeds the maximum allowed characters (%d): field truncated", MaxScriptNameRunes),
				)
			}
			evaluatedJob.Name = name
		}

		if job.Description != nil {
			desc, err := evalScriptStringField(evalctx, job.Description.Expr, "script.job.description")
			errs.Append(err)
			if len(desc) > MaxScriptDescRunes {
				desc = desc[:MaxScriptDescRunes]

				printer.Stderr.Warn(
					fmt.Sprintf("`script.job.description` exceeds the maximum allowed characters (%d): field truncated", MaxScriptDescRunes),
				)
			}

			evaluatedJob.Description = desc
		}

		if job.Command != nil {
			expr := job.Command.Expr
			v, err := evalctx.Eval(expr)
			if err != nil {
				errs.Append(errors.E(ErrScriptSchema, expr.Range(), err, "evaluating command"))
				continue
			}

			command, err := unmarshalScriptJobCommand(v, expr)
			if err != nil {
				errs.Append(err)
				continue
			}
			evaluatedJob.Cmd = command
		}

		if job.Commands != nil {
			expr := job.Commands.Expr
			v, err := evalctx.Eval(expr)
			if err != nil {
				errs.Append(errors.E(ErrScriptSchema, expr.Range(), err, "evaluating commands"))
				continue
			}

			commands, err := unmarshalScriptJobCommands(v, expr)
			if err != nil {
				errs.Append(err)
				continue
			}
			evaluatedJob.Cmds = commands
		}

		evaluatedScript.Jobs = append(evaluatedScript.Jobs, evaluatedJob)
	}

	// Validate option constraints
	var cmdsWithCloudSyncDeployment []string
	for jobIdx, job := range evaluatedScript.Jobs {
		for cmdIdx, cmd := range job.Commands() {
			if cmd.Options != nil && cmd.Options.CloudSyncDeployment {
				cmdsWithCloudSyncDeployment = append(cmdsWithCloudSyncDeployment, fmt.Sprintf("job:%d.%d", jobIdx, cmdIdx))
			}
		}
	}
	if len(cmdsWithCloudSyncDeployment) > 1 {
		errs.Append(errors.E(ErrScriptInvalidCmdOptions,
			"only a single command per script may have 'cloud_sync_deployment' enabled, but was enabled by: %v",
			strings.Join(cmdsWithCloudSyncDeployment, " "),
		))
	}

	var cmdsWithCloudSyncPreview []string
	for jobIdx, job := range evaluatedScript.Jobs {
		for cmdIdx, cmd := range job.Commands() {
			if cmd.Options != nil && cmd.Options.CloudSyncPreview {
				cmdsWithCloudSyncPreview = append(cmdsWithCloudSyncPreview, fmt.Sprintf("job:%d.%d", jobIdx, cmdIdx))
			}
		}
	}
	if len(cmdsWithCloudSyncPreview) > 1 {
		errs.Append(errors.E(ErrScriptInvalidCmdOptions,
			"only a single command per script may have 'cloud_sync_preview' enabled, but was enabled by: %v",
			strings.Join(cmdsWithCloudSyncDeployment, " "),
		))
	}

	if err := errs.AsError(); err != nil {
		return Script{}, err
	}

	return evaluatedScript, nil
}

func evalScriptStringField(evalctx *eval.Context, expr hhcl.Expression, name string) (string, error) {
	f, err := evalString(evalctx, expr, name)
	if err != nil {
		return "", errors.E(ErrScriptInvalidType, expr.Range(), err)
	}
	return f, nil
}

func unmarshalScriptJobCommands(cmdList cty.Value, expr hhcl.Expression) ([]*ScriptCmd, error) {
	if !cmdList.Type().IsTupleType() && !cmdList.Type().IsListType() {
		return nil, errors.E(ErrScriptInvalidTypeCommands,
			expr.Range(), "commands should be a list but got %s", cmdList.Type().FriendlyName())
	}

	if cmdList.LengthInt() == 0 {
		return nil, errors.E(ErrScriptEmptyCmds, expr.Range())
	}

	errs := errors.L()
	evaluatedCommands := []*ScriptCmd{}

	index := -1
	it := cmdList.ElementIterator()
	for it.Next() {
		index++
		_, elem := it.Element()
		if !elem.Type().IsTupleType() && !elem.Type().IsListType() {
			errs.Append(errors.E(ErrScriptInvalidTypeCommands, expr.Range(),
				"commands must be a list of list, but element %d has type %q",
				index, elem.Type().FriendlyName()))
			continue
		}

		if elem.LengthInt() == 0 {
			errs.Append(errors.E(ErrScriptEmptyCmds, expr.Range(), "commands item %d is empty", index))
			continue
		}

		evaluatedCommand, err := unmarshalScriptJobCommand(elem, expr)
		if err != nil {
			errs.Append(err)
			continue
		}

		evaluatedCommands = append(evaluatedCommands, evaluatedCommand)
	}

	if err := errs.AsError(); err != nil {
		return nil, err
	}

	return evaluatedCommands, nil
}

func unmarshalScriptJobCommand(cmdValues cty.Value, expr hhcl.Expression) (*ScriptCmd, error) {
	if !cmdValues.Type().IsTupleType() && !cmdValues.Type().IsListType() {
		return nil, errors.E(ErrScriptInvalidTypeCommand, expr.Range(), "command must be a list but got %s",
			cmdValues.Type().FriendlyName())
	}

	if cmdValues.LengthInt() == 0 {
		return nil, errors.E(ErrScriptEmptyCmds, expr.Range())
	}

	errs := errors.L()
	r := &ScriptCmd{}

	index := 0
	lastIndex := cmdValues.LengthInt() - 1

	it := cmdValues.ElementIterator()
	for it.Next() {
		_, elem := it.Element()
		if elem.Type() == cty.String {
			r.Args = append(r.Args, elem.AsString())
		} else if index == lastIndex {
			if elem.Type().IsObjectType() {
				var err error
				r.Options, err = unmarshalScriptCommandOptions(elem, expr)
				if r.Options != nil &&
					r.Options.CloudSyncPreview &&
					(r.Options.CloudSyncDriftStatus || r.Options.CloudSyncDeployment) {
					errs.Append(errors.E(ErrScriptInvalidCmdOptions, expr.Range(),
						"cloud_sync_preview cannot be used with cloud_sync_deployment or cloud_sync_drift_status"))
				}
				errs.Append(err)
			} else {
				errs.Append(errors.E(ErrScriptInvalidTypeCommand, expr.Range(),
					"command options must be an object, but last element has type %s",
					elem.Type().FriendlyName()))
			}
		} else {
			errs.Append(errors.E(ErrScriptInvalidTypeCommand, expr.Range(),
				"command must be a list(string), but element %d has type %s",
				index, elem.Type().FriendlyName()))
		}

		index++
	}

	if err := errs.AsError(); err != nil {
		return nil, err
	}

	return r, nil
}

func unmarshalScriptCommandOptions(obj cty.Value, expr hhcl.Expression) (*ScriptCmdOptions, error) {
	r := &ScriptCmdOptions{}
	it := obj.ElementIterator()

	errs := errors.L()

	for it.Next() {
		k, v := it.Element()

		if k.Type() != cty.String {
			errs.Append(errors.E(ErrScriptInvalidCmdOptions, expr.Range(),
				"command option key must be a string, but has type %s",
				k.Type().FriendlyName()))
			continue
		}

		switch ks := k.AsString(); ks {
		case "cloud_sync_deployment":
			if v.Type() != cty.Bool {
				errs.Append(errors.E(ErrScriptInvalidCmdOptions, expr.Range(),
					"command option '%s' must be a bool, but has type %s",
					ks, v.Type().FriendlyName()))
				break
			}
			r.CloudSyncDeployment = v.True()

		case "cloud_sync_drift_status":
			if v.Type() != cty.Bool {
				errs.Append(errors.E(ErrScriptInvalidCmdOptions, expr.Range(),
					"command option '%s' must be a bool, but has type %s",
					ks, v.Type().FriendlyName()))
				break
			}
			r.CloudSyncDriftStatus = v.True()
		case "cloud_sync_preview":
			if v.Type() != cty.Bool {
				errs.Append(errors.E(ErrScriptInvalidCmdOptions, expr.Range(),
					"command option '%s' must be a bool, but has type %s",
					ks, v.Type().FriendlyName()))
				break
			}
			r.CloudSyncPreview = v.True()

		case "cloud_sync_layer":
			if v.Type() != cty.String {
				errs.Append(errors.E(ErrScriptInvalidCmdOptions, expr.Range(),
					"command option '%s' must be a string, but has type %s",
					ks, v.Type().FriendlyName()))
				break
			}

			r.CloudSyncLayer = preview.Layer(v.AsString())
			if r.CloudSyncLayer.Validate() != nil {
				errs.Append(errors.E(ErrScriptInvalidCmdOptions, expr.Range(),
					"command option '%s' must contain only alphanumeric characters and hyphens", ks))
			}

		case "cloud_sync_terraform_plan_file":
			if v.Type() != cty.String {
				errs.Append(errors.E(ErrScriptInvalidCmdOptions, expr.Range(),
					"command option '%s' must be a string, but has type %s",
					ks, v.Type().FriendlyName()))
				break
			}
			r.CloudSyncTerraformPlan = v.AsString()

		case "terragrunt":
			if v.Type() != cty.Bool {
				errs.Append(errors.E(ErrScriptInvalidCmdOptions, expr.Range(),
					"command option '%s' must be a bool, but has type %s",
					ks, v.Type().FriendlyName()))
				break
			}
			r.UseTerragrunt = v.True()

		default:
			errs.Append(errors.E(ErrScriptInvalidCmdOptions, expr.Range(), "unknown command option: %s", ks))
		}

		if r.CloudSyncDeployment && r.CloudSyncDriftStatus {
			errs.Append(errors.E(ErrScriptInvalidCmdOptions, expr.Range(),
				"command option 'cloud_sync_deployment' and 'cloud_sync_drift_status' are conflicting options in the same command"))
		}
	}

	if err := errs.AsError(); err != nil {
		return nil, err
	}

	return r, nil
}
