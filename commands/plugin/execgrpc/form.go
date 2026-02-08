// Copyright 2025 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package execgrpc

import (
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/huh"
	"github.com/terramate-io/terramate/commands"
	"github.com/terramate-io/terramate/errors"
	pb "github.com/terramate-io/terramate/plugin/proto/v1"
	"golang.org/x/term"
)

type fieldResult struct {
	id  string
	get func() (string, error)
}

func renderForm(cli commands.CLI, req *pb.FormRequest) (*pb.FormResponse, error) {
	if req == nil {
		return nil, errors.E("form request is required")
	}
	if envVarIsSet(os.Getenv("TM_FORM_AUTOFILL_DEFAULTS")) {
		return autofillForm(req), nil
	}

	fields := make([]huh.Field, 0, len(req.Fields))
	results := make([]fieldResult, 0, len(req.Fields))

	for _, field := range req.Fields {
		if field == nil {
			continue
		}
		rendered, res, err := buildField(field)
		if err != nil {
			return nil, err
		}
		fields = append(fields, rendered)
		results = append(results, res)
	}

	form := huh.NewForm(huh.NewGroup(fields...)).
		WithInput(cli.Stdin()).
		WithOutput(cli.Stdout())

	ttyFile, ok := cli.Stdin().(*os.File)
	var oldState *term.State
	if ok && term.IsTerminal(int(ttyFile.Fd())) {
		oldState, _ = term.GetState(int(ttyFile.Fd()))
	}

	if err := form.Run(); err != nil {
		if oldState != nil {
			_ = term.Restore(int(ttyFile.Fd()), oldState)
		}
		if errors.Is(err, huh.ErrUserAborted) {
			return &pb.FormResponse{
				Id:        req.Id,
				Cancelled: true,
			}, nil
		}
		return nil, err
	}
	if oldState != nil {
		_ = term.Restore(int(ttyFile.Fd()), oldState)
	}

	values := make(map[string]string, len(results))
	for _, res := range results {
		val, err := res.get()
		if err != nil {
			return nil, err
		}
		values[res.id] = val
	}

	return &pb.FormResponse{
		Id:     req.Id,
		Values: values,
	}, nil
}

func buildField(field *pb.FormField) (huh.Field, fieldResult, error) {
	switch typed := field.FieldType.(type) {
	case *pb.FormField_Select:
		return buildSelectField(field, typed.Select)
	case *pb.FormField_MultiSelect:
		return buildMultiSelectField(field, typed.MultiSelect)
	case *pb.FormField_TextInput:
		return buildTextInputField(field)
	case *pb.FormField_TextArea:
		return buildTextAreaField(field)
	case *pb.FormField_Confirm:
		return buildConfirmField(field, typed.Confirm)
	default:
		return nil, fieldResult{}, errors.E("unsupported form field type")
	}
}

func buildSelectField(field *pb.FormField, spec *pb.SelectFormField) (huh.Field, fieldResult, error) {
	var selected string
	if field.DefaultValue != "" {
		selected = field.DefaultValue
	}

	options := make([]huh.Option[string], 0, len(spec.Options))
	for _, opt := range spec.Options {
		options = append(options, huh.NewOption(opt.Label, opt.Value))
	}

	input := huh.NewSelect[string]().
		Title(field.Title).
		Options(options...).
		Value(&selected)

	if field.Description != "" {
		input = input.Description(field.Description)
	}
	if field.Required {
		input = input.Validate(requiredString(field.Title))
	}

	return input, fieldResult{
		id: field.Id,
		get: func() (string, error) {
			return selected, nil
		},
	}, nil
}

func buildMultiSelectField(field *pb.FormField, spec *pb.MultiSelectFormField) (huh.Field, fieldResult, error) {
	var selected []string
	if field.DefaultValue != "" {
		selected = splitCSV(field.DefaultValue)
	}

	options := make([]huh.Option[string], 0, len(spec.Options))
	for _, opt := range spec.Options {
		options = append(options, huh.NewOption(opt.Label, opt.Value))
	}

	input := huh.NewMultiSelect[string]().
		Title(field.Title).
		Options(options...).
		Value(&selected)

	if field.Description != "" {
		input = input.Description(field.Description)
	}
	if field.Required {
		input = input.Validate(func(values []string) error {
			if len(values) == 0 {
				return errors.E("this value is required")
			}
			return nil
		})
	}

	return input, fieldResult{
		id: field.Id,
		get: func() (string, error) {
			return strings.Join(selected, ","), nil
		},
	}, nil
}

func buildTextInputField(field *pb.FormField) (huh.Field, fieldResult, error) {
	value := field.DefaultValue

	input := huh.NewInput().
		Title(field.Title).
		Value(&value)

	if field.Placeholder != "" {
		input = input.Placeholder(field.Placeholder)
	}
	if field.Description != "" {
		input = input.Description(field.Description)
	}
	if field.Required {
		input = input.Validate(requiredString(field.Title))
	}

	return input, fieldResult{
		id: field.Id,
		get: func() (string, error) {
			return value, nil
		},
	}, nil
}

func buildTextAreaField(field *pb.FormField) (huh.Field, fieldResult, error) {
	value := field.DefaultValue

	input := huh.NewText().
		Title(field.Title).
		Value(&value).
		ExternalEditor(false)

	if field.Placeholder != "" {
		input = input.Placeholder(field.Placeholder)
	}
	if field.Description != "" {
		input = input.Description(field.Description)
	}
	if field.Required {
		input = input.Validate(requiredString(field.Title))
	}

	return input, fieldResult{
		id: field.Id,
		get: func() (string, error) {
			return value, nil
		},
	}, nil
}

func buildConfirmField(field *pb.FormField, spec *pb.ConfirmFormField) (huh.Field, fieldResult, error) {
	var value bool
	if field.DefaultValue != "" {
		switch strings.ToLower(strings.TrimSpace(field.DefaultValue)) {
		case "true", "1", "yes":
			value = true
		}
	}

	input := huh.NewConfirm().
		Title(field.Title).
		Value(&value)

	if field.Description != "" {
		input = input.Description(field.Description)
	}
	if spec != nil {
		if spec.Affirmative != "" {
			input = input.Affirmative(spec.Affirmative)
		}
		if spec.Negative != "" {
			input = input.Negative(spec.Negative)
		}
	}

	return input, fieldResult{
		id: field.Id,
		get: func() (string, error) {
			if value {
				return "true", nil
			}
			return "false", nil
		},
	}, nil
}

func requiredString(label string) func(string) error {
	return func(val string) error {
		if strings.TrimSpace(val) == "" {
			if label == "" {
				return errors.E("this value is required")
			}
			return fmt.Errorf("%s is required", label)
		}
		return nil
	}
}

func splitCSV(value string) []string {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	parts := strings.Split(value, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed == "" {
			continue
		}
		out = append(out, trimmed)
	}
	return out
}

func envVarIsSet(val string) bool {
	v := strings.TrimSpace(strings.ToLower(val))
	return v != "" && v != "0" && v != "false"
}

func autofillForm(req *pb.FormRequest) *pb.FormResponse {
	values := make(map[string]string, len(req.Fields))
	for _, field := range req.Fields {
		if field == nil {
			continue
		}
		switch typed := field.FieldType.(type) {
		case *pb.FormField_Select:
			values[field.Id] = selectDefaultValue(field, typed.Select)
		case *pb.FormField_MultiSelect:
			values[field.Id] = selectDefaultValue(field, typed.MultiSelect)
		case *pb.FormField_TextInput, *pb.FormField_TextArea:
			if field.DefaultValue != "" {
				values[field.Id] = field.DefaultValue
			} else if field.Required {
				values[field.Id] = ""
			}
		case *pb.FormField_Confirm:
			if field.DefaultValue != "" {
				values[field.Id] = field.DefaultValue
			} else if field.Required {
				values[field.Id] = "true"
			}
		default:
		}
	}

	return &pb.FormResponse{
		Id:     req.Id,
		Values: values,
	}
}

func selectDefaultValue(field *pb.FormField, spec interface{ GetOptions() []*pb.FormOption }) string {
	if field.DefaultValue != "" {
		return field.DefaultValue
	}
	if field.Required {
		options := spec.GetOptions()
		if len(options) > 0 {
			return options[0].Value
		}
	}
	return ""
}
