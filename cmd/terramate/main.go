// Copyright 2023 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

// Terramate is a tool for managing multiple Terraform stacks. Providing stack
// execution orchestration and code generation as a way to share data across
// different stacks.
// For details on how to use it just run:
//
//	terramate --help
package main

import (
	"context"
	"fmt"
	"os"

	tmerrors "github.com/terramate-io/terramate/errors"
	"github.com/terramate-io/terramate/hcl"
	"github.com/terramate-io/terramate/plugin"
	plugingrpc "github.com/terramate-io/terramate/plugin/grpc"
	pb "github.com/terramate-io/terramate/plugin/proto/v1"
	"github.com/terramate-io/terramate/ui/tui"
)

func main() {
	opts := []tui.Option{}
	if !grpcPluginsDisabled() {
		userDir, err := plugin.ResolveUserTerramateDir()
		if err == nil {
			installed, err := plugingrpc.DiscoverInstalled(userDir)
			if err == nil && len(installed) > 0 {
				specs := make([]tui.PluginCommandSpec, 0, len(installed))
				var allHCLOptions []hcl.Option
				for _, plg := range installed {
					client, err := plugingrpc.NewHostClient(plg.BinaryPath)
					if err != nil {
						fmt.Fprintln(os.Stderr, err)
						continue
					}
					grpcClient := client.Client()

					// Discover commands
					resp, err := grpcClient.CommandService.GetCommands(context.Background(), &pb.Empty{})
					if err != nil {
						fmt.Fprintln(os.Stderr, err)
						client.Kill()
						continue
					}
					specs = append(specs, tui.PluginCommandSpec{
						PluginName: plg.Manifest.Name,
						BinaryPath: plg.BinaryPath,
						Commands:   resp.Commands,
					})

					// Discover HCL schemas
					schemaResp, err := grpcClient.HCLSchemaService.GetHCLSchema(context.Background(), &pb.Empty{})
					client.Kill()
					if err != nil {
						fmt.Fprintln(os.Stderr, err)
						continue
					}
					hclOpts := plugingrpc.HCLOptionsFromSchema(plg.Manifest.Name, plg.BinaryPath, schemaResp.Blocks)
					if len(hclOpts) > 0 {
						allHCLOptions = append(allHCLOptions, hclOpts...)
					}
				}
				if len(specs) > 0 {
					opts = append(opts, tui.WithPluginCommands(specs))
				}
				if len(allHCLOptions) > 0 {
					opts = append(opts, tui.WithHCLOptions(allHCLOptions...))
				}
			}
		}
	}

	cli, err := tui.NewCLI(opts...)
	if err != nil {
		panic(tmerrors.E(tmerrors.ErrInternal, "unexpected error"))
	}

	cli.Exec(os.Args[1:])
}

func grpcPluginsDisabled() bool {
	val := os.Getenv("TM_DISABLE_GRPC_PLUGINS")
	return val != "" && val != "0" && val != "false"
}
