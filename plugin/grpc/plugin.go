// Copyright 2025 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package grpc

import (
	"context"
	"errors"

	"github.com/hashicorp/go-plugin"
	pb "github.com/terramate-io/terramate/plugin/proto/v1"
	"google.golang.org/grpc"
)

const grpcPluginName = "terramate-grpc"

// Server holds the gRPC services provided by a plugin binary.
type Server struct {
	PluginService    pb.PluginServiceServer
	CommandService   pb.CommandServiceServer
	HCLSchemaService pb.HCLSchemaServiceServer
	LifecycleService pb.LifecycleServiceServer
	GenerateService  pb.GenerateServiceServer
}

// Client provides typed access to plugin services over gRPC.
type Client struct {
	PluginService    pb.PluginServiceClient
	CommandService   pb.CommandServiceClient
	HCLSchemaService pb.HCLSchemaServiceClient
	LifecycleService pb.LifecycleServiceClient
	GenerateService  pb.GenerateServiceClient

	conn *grpc.ClientConn
}

// Close releases the underlying gRPC connection.
func (c *Client) Close() error {
	if c == nil || c.conn == nil {
		return nil
	}
	return c.conn.Close()
}

// PluginMap returns the go-plugin map for the provided server.
func PluginMap(server *Server) map[string]plugin.Plugin {
	return map[string]plugin.Plugin{
		grpcPluginName: &GRPCPlugin{Services: server},
	}
}

// GRPCPlugin wires Terramate's gRPC services into go-plugin.
//
//revive:disable-next-line:exported
type GRPCPlugin struct {
	plugin.NetRPCUnsupportedPlugin
	Services *Server
}

// GRPCServer registers plugin services on the gRPC server.
func (p *GRPCPlugin) GRPCServer(_ *plugin.GRPCBroker, s *grpc.Server) error {
	if p.Services == nil {
		return errors.New("grpc plugin server is not configured")
	}
	if p.Services.PluginService != nil {
		pb.RegisterPluginServiceServer(s, p.Services.PluginService)
	}
	if p.Services.CommandService != nil {
		pb.RegisterCommandServiceServer(s, p.Services.CommandService)
	}
	if p.Services.HCLSchemaService != nil {
		pb.RegisterHCLSchemaServiceServer(s, p.Services.HCLSchemaService)
	}
	if p.Services.LifecycleService != nil {
		pb.RegisterLifecycleServiceServer(s, p.Services.LifecycleService)
	}
	if p.Services.GenerateService != nil {
		pb.RegisterGenerateServiceServer(s, p.Services.GenerateService)
	}
	return nil
}

// GRPCClient constructs a typed client for plugin services.
func (p *GRPCPlugin) GRPCClient(_ context.Context, _ *plugin.GRPCBroker, cc *grpc.ClientConn) (interface{}, error) {
	return &Client{
		PluginService:    pb.NewPluginServiceClient(cc),
		CommandService:   pb.NewCommandServiceClient(cc),
		HCLSchemaService: pb.NewHCLSchemaServiceClient(cc),
		LifecycleService: pb.NewLifecycleServiceClient(cc),
		GenerateService:  pb.NewGenerateServiceClient(cc),
		conn:             cc,
	}, nil
}
