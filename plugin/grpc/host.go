// Copyright 2025 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package grpc

import (
	"fmt"
	"io"
	"os"
	"os/exec"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/go-plugin"
)

// HostAddrEnv is the environment variable used to pass host service address.
const HostAddrEnv = "TM_PLUGIN_HOST_ADDR"

// HostClient manages a plugin subprocess and its gRPC client.
type HostClient struct {
	client *plugin.Client
	grpc   *Client
}

// NewHostClient starts the plugin binary and establishes a gRPC connection.
func NewHostClient(binaryPath string) (*HostClient, error) {
	return NewHostClientWithEnv(binaryPath, nil)
}

// NewHostClientWithEnv starts the plugin binary with extra env vars.
func NewHostClientWithEnv(binaryPath string, extraEnv []string) (*HostClient, error) {
	cmd := exec.Command(binaryPath)
	cmd.Env = append(os.Environ(), extraEnv...)
	cfg := &plugin.ClientConfig{
		HandshakeConfig:  HandshakeConfig(),
		Plugins:          PluginMap(nil),
		Cmd:              cmd,
		AllowedProtocols: []plugin.Protocol{plugin.ProtocolGRPC},
		AutoMTLS:         true,
		Managed:          true,
		Logger:           hclog.New(&hclog.LoggerOptions{Output: io.Discard, Level: hclog.Off}),
	}

	client := plugin.NewClient(cfg)
	rpcClient, err := client.Client()
	if err != nil {
		client.Kill()
		return nil, err
	}

	raw, err := rpcClient.Dispense(grpcPluginName)
	if err != nil {
		client.Kill()
		return nil, err
	}

	grpcClient, ok := raw.(*Client)
	if !ok {
		client.Kill()
		return nil, fmt.Errorf("unexpected gRPC plugin client type %T", raw)
	}

	return &HostClient{
		client: client,
		grpc:   grpcClient,
	}, nil
}

// Client returns the typed gRPC client.
func (c *HostClient) Client() *Client {
	if c == nil {
		return nil
	}
	return c.grpc
}

// Kill stops the plugin process and releases resources.
func (c *HostClient) Kill() {
	if c == nil || c.client == nil {
		return
	}
	c.client.Kill()
}
