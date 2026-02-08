// Copyright 2025 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package tui

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"

	"github.com/alecthomas/kong"
	pb "github.com/terramate-io/terramate/plugin/proto/v1"
)

// PluginCommandSpec describes a plugin's top-level command specs.
type PluginCommandSpec struct {
	PluginName string
	BinaryPath string
	Commands   []*pb.CommandSpec
}

// PluginCommand identifies a resolved plugin command for execution.
type PluginCommand struct {
	PluginName string
	BinaryPath string
	Command    string
	Spec       *pb.CommandSpec
}

// WithPluginCommands registers plugin commands into the CLI parser.
func WithPluginCommands(specs []PluginCommandSpec) Option {
	return func(c *CLI) error {
		if len(specs) == 0 {
			return nil
		}

		if c.pluginCommands == nil {
			c.pluginCommands = map[string]PluginCommand{}
		}

		for _, pluginSpec := range specs {
			for _, cmd := range pluginSpec.Commands {
				cmdInstance, err := buildCommandInstance(cmd)
				if err != nil {
					return err
				}
				c.kongDynamicOptions = append(
					c.kongDynamicOptions,
					kong.DynamicCommand(cmd.Name, cmd.Help, "plugins", cmdInstance),
				)

				registerPluginCommands(c.pluginCommands, pluginSpec, cmd, cmd.Name)
			}
		}
		return nil
	}
}

func (c *CLI) pluginCommand(name string) (PluginCommand, bool) {
	if c == nil || c.pluginCommands == nil {
		return PluginCommand{}, false
	}
	cmd, ok := c.pluginCommands[name]
	return cmd, ok
}

func registerPluginCommands(registry map[string]PluginCommand, pluginSpec PluginCommandSpec, cmd *pb.CommandSpec, path string) {
	if len(cmd.Subcommands) == 0 {
		registry[path] = PluginCommand{
			PluginName: pluginSpec.PluginName,
			BinaryPath: pluginSpec.BinaryPath,
			Command:    path,
			Spec:       cmd,
		}
		return
	}
	for _, sub := range cmd.Subcommands {
		subPath := path + " " + sub.Name
		registerPluginCommands(registry, pluginSpec, sub, subPath)
	}
}

func buildCommandInstance(cmd *pb.CommandSpec) (any, error) {
	typ, err := buildCommandType(cmd)
	if err != nil {
		return nil, err
	}
	return reflect.New(typ).Interface(), nil
}

func buildCommandType(cmd *pb.CommandSpec) (reflect.Type, error) {
	fields := make([]reflect.StructField, 0, len(cmd.Flags)+len(cmd.Args)+len(cmd.Subcommands))

	for _, flag := range cmd.Flags {
		fieldType, err := flagType(flag.Type)
		if err != nil {
			return nil, err
		}
		tag := structTagFromFlag(flag)
		fields = append(fields, reflect.StructField{
			Name: exportFieldName(flag.Name),
			Type: fieldType,
			Tag:  reflect.StructTag(tag),
		})
	}

	for _, arg := range cmd.Args {
		tag := structTagFromArg(arg)
		fields = append(fields, reflect.StructField{
			Name: exportFieldName(arg.Name),
			Type: reflect.TypeOf(""),
			Tag:  reflect.StructTag(tag),
		})
	}

	for _, sub := range cmd.Subcommands {
		subType, err := buildCommandType(sub)
		if err != nil {
			return nil, err
		}
		tag := fmt.Sprintf(`cmd:"" help:"%s"`, escapeTag(sub.Help))
		fields = append(fields, reflect.StructField{
			Name: exportFieldName(sub.Name),
			Type: subType,
			Tag:  reflect.StructTag(tag),
		})
	}

	return reflect.StructOf(fields), nil
}

func flagType(kind string) (reflect.Type, error) {
	switch strings.ToLower(kind) {
	case "", "string", "enum":
		return reflect.TypeOf(""), nil
	case "bool":
		return reflect.TypeOf(true), nil
	case "int":
		return reflect.TypeOf(int(0)), nil
	default:
		return nil, fmt.Errorf("unsupported flag type %q", kind)
	}
}

func structTagFromFlag(flag *pb.CommandFlag) string {
	parts := []string{
		fmt.Sprintf(`name:"%s"`, escapeTag(flag.Name)),
	}
	if flag.Help != "" {
		parts = append(parts, fmt.Sprintf(`help:"%s"`, escapeTag(flag.Help)))
	}
	if flag.Short != "" {
		parts = append(parts, fmt.Sprintf(`short:"%s"`, escapeTag(flag.Short)))
	}
	if flag.DefaultValue != "" {
		parts = append(parts, fmt.Sprintf(`default:"%s"`, escapeTag(flag.DefaultValue)))
	}
	if len(flag.EnumValues) > 0 {
		parts = append(parts, fmt.Sprintf(`enum:"%s"`, strings.Join(flag.EnumValues, ",")))
	}
	return strings.Join(parts, " ")
}

func structTagFromArg(arg *pb.CommandArg) string {
	parts := []string{
		`arg:""`,
		fmt.Sprintf(`name:"%s"`, escapeTag(arg.Name)),
	}
	if arg.Help != "" {
		parts = append(parts, fmt.Sprintf(`help:"%s"`, escapeTag(arg.Help)))
	}
	if !arg.Required {
		parts = append(parts, `optional:"true"`)
	}
	return strings.Join(parts, " ")
}

func escapeTag(val string) string {
	return strings.ReplaceAll(val, `"`, `\"`)
}

func exportFieldName(name string) string {
	parts := strings.FieldsFunc(name, func(r rune) bool {
		return r == '-' || r == '_' || r == ' '
	})
	for i, part := range parts {
		if part == "" {
			continue
		}
		parts[i] = strings.ToUpper(part[:1]) + part[1:]
	}
	out := strings.Join(parts, "")
	if out == "" {
		return "Field"
	}
	if out[0] >= '0' && out[0] <= '9' {
		return "Field" + out
	}
	return out
}

func encodeKongValue(val reflect.Value) string {
	if !val.IsValid() {
		return ""
	}
	if val.Kind() == reflect.Pointer {
		if val.IsNil() {
			return ""
		}
		val = val.Elem()
	}
	switch val.Kind() {
	case reflect.Slice, reflect.Map, reflect.Struct, reflect.Array:
		b, err := json.Marshal(val.Interface())
		if err == nil {
			return string(b)
		}
	}
	return fmt.Sprint(val.Interface())
}

func extractCommandValues(kctx *kong.Context) (map[string]string, map[string]string) {
	args := map[string]string{}
	flags := map[string]string{}

	if kctx == nil {
		return args, flags
	}

	selected := kctx.Selected()
	if selected == nil {
		return args, flags
	}

	commandNodes := map[*kong.Node]struct{}{}
	for node := selected; node != nil; node = node.Parent {
		commandNodes[node] = struct{}{}
	}

	for _, path := range kctx.Path {
		if path.Flag != nil {
			if path.Parent != nil {
				if _, ok := commandNodes[path.Parent]; !ok {
					continue
				}
			}
			flags[path.Flag.Name] = encodeKongValue(kctx.Value(path))
			continue
		}

		if path.Positional != nil {
			if path.Parent != nil {
				if _, ok := commandNodes[path.Parent]; !ok {
					continue
				}
			}
			args[path.Positional.Name] = encodeKongValue(kctx.Value(path))
		}
	}

	return args, flags
}
