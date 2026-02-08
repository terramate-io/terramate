// Copyright 2023 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

// Terramate-ls is a language server.
// For details on how to use it just run:
//
//	terramate-ls --help
package main

import (
	"context"
	"fmt"
	"os"

	"github.com/terramate-io/terramate/hcl"
	tmls "github.com/terramate-io/terramate/ls"
	"github.com/terramate-io/terramate/plugin"
	plugingrpc "github.com/terramate-io/terramate/plugin/grpc"
	pb "github.com/terramate-io/terramate/plugin/proto/v1"
)

func main() {
	var opts []tmls.Option
	if !grpcPluginsDisabled() {
		userDir, err := plugin.ResolveUserTerramateDir()
		if err == nil {
			installed, err := plugingrpc.DiscoverInstalled(userDir)
			if err != nil {
				fmt.Fprintln(os.Stderr, err)
			} else if len(installed) > 0 {
				var allHCLOptions []hcl.Option
				for _, plg := range installed {
					client, err := plugingrpc.NewHostClient(plg.BinaryPath)
					if err != nil {
						fmt.Fprintln(os.Stderr, err)
						continue
					}
					grpcClient := client.Client()
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
				if len(allHCLOptions) > 0 {
					opts = append(opts, tmls.WithHCLOptions(allHCLOptions...))
				}
			}
		}
	}

	if len(opts) > 0 {
		tmls.RunServer(opts...)
		return
	}
	tmls.RunServer()
}

func grpcPluginsDisabled() bool {
	val := os.Getenv("TM_DISABLE_GRPC_PLUGINS")
	return val != "" && val != "0" && val != "false"
}
