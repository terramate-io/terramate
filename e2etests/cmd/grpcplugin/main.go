// Copyright 2025 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

// Package main provides a test gRPC plugin binary for e2e coverage.
package main

import (
	"context"
	"fmt"
	"os"

	"github.com/hashicorp/go-plugin"
	plugingrpc "github.com/terramate-io/terramate/plugin/grpc"
	pb "github.com/terramate-io/terramate/plugin/proto/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func main() {
	plugin.Serve(&plugin.ServeConfig{
		HandshakeConfig: plugingrpc.HandshakeConfig(),
		Plugins: plugingrpc.PluginMap(&plugingrpc.Server{
			PluginService:    &grpcPluginService{},
			CommandService:   &grpcCommandService{},
			HCLSchemaService: &grpcHCLSchemaService{},
			LifecycleService: &grpcLifecycleService{},
			GenerateService:  &grpcGenerateService{},
		}),
		GRPCServer: plugin.DefaultGRPCServer,
	})
}

type grpcPluginService struct {
	pb.UnimplementedPluginServiceServer
}

func (s *grpcPluginService) GetPluginInfo(context.Context, *pb.Empty) (*pb.PluginInfo, error) {
	return &pb.PluginInfo{
		Name:              "testgrpc",
		Version:           "0.1.0",
		ProductName:       "terramate-grpc-test-plugin",
		ProductPrettyName: "Terramate gRPC Test Plugin",
		Description:       "Test plugin for Terramate gRPC e2e coverage",
		CompatibleWith:    "terramate",
	}, nil
}

func (s *grpcPluginService) GetCapabilities(context.Context, *pb.Empty) (*pb.Capabilities, error) {
	return &pb.Capabilities{
		HasCommands:         true,
		HasHclSchema:        true,
		HasPostInitHooks:    true,
		HasGenerateOverride: true,
	}, nil
}

func (s *grpcPluginService) Shutdown(context.Context, *pb.Empty) (*pb.Empty, error) {
	return &pb.Empty{}, nil
}

type grpcCommandService struct {
	pb.UnimplementedCommandServiceServer
}

func (s *grpcCommandService) GetCommands(context.Context, *pb.Empty) (*pb.CommandList, error) {
	return &pb.CommandList{
		Commands: []*pb.CommandSpec{
			{
				Name: "hello",
				Help: "emit a hello message",
			},
			{
				Name: "fail",
				Help: "emit a failure exit code",
			},
		},
	}, nil
}

func (s *grpcCommandService) ExecuteCommand(req *pb.CommandRequest, stream pb.CommandService_ExecuteCommandServer) error {
	if req == nil {
		return fmt.Errorf("command request is required")
	}
	switch req.Command {
	case "hello":
		if err := stream.Send(&pb.CommandOutput{
			Output: &pb.CommandOutput_Stdout{Stdout: []byte("hello\n")},
		}); err != nil {
			return err
		}
		return stream.Send(&pb.CommandOutput{Output: &pb.CommandOutput_ExitCode{ExitCode: 0}})
	case "fail":
		if err := stream.Send(&pb.CommandOutput{
			Output: &pb.CommandOutput_Stderr{Stderr: []byte("plugin failure\n")},
		}); err != nil {
			return err
		}
		return stream.Send(&pb.CommandOutput{Output: &pb.CommandOutput_ExitCode{ExitCode: 2}})
	default:
		return fmt.Errorf("unknown command %q", req.Command)
	}
}

type grpcHCLSchemaService struct {
	pb.UnimplementedHCLSchemaServiceServer
}

func (s *grpcHCLSchemaService) GetHCLSchema(context.Context, *pb.Empty) (*pb.HCLSchemaList, error) {
	return &pb.HCLSchemaList{
		Blocks: []*pb.HCLBlockSchema{
			{
				Name:       "pluginblock",
				Kind:       pb.BlockKind_BLOCK_MERGED_LABELS,
				LabelCount: 1,
				Attributes: []*pb.HCLAttributeSchema{
					{Name: "value", Type: "string", Required: true},
				},
			},
		},
	}, nil
}

func (s *grpcHCLSchemaService) ProcessParsedBlocks(_ context.Context, req *pb.ParsedBlocksRequest) (*pb.ParsedBlocksResponse, error) {
	marker := os.Getenv("TM_E2E_GRPC_MARKER")
	if marker != "" && req != nil {
		_ = os.WriteFile(marker, []byte(fmt.Sprintf("%s:%d", req.BlockType, len(req.Blocks))), 0o644)
	}
	return &pb.ParsedBlocksResponse{}, nil
}

type grpcLifecycleService struct {
	pb.UnimplementedLifecycleServiceServer
}

func (s *grpcLifecycleService) PostInit(ctx context.Context, _ *pb.PostInitRequest) (*pb.PostInitResponse, error) {
	marker := os.Getenv("TM_E2E_GRPC_POSTINIT_MARKER")
	stackName := os.Getenv("TM_E2E_GRPC_STACK_NAME")
	if marker == "" && stackName == "" {
		return &pb.PostInitResponse{}, nil
	}

	return &pb.PostInitResponse{}, withHostClient(ctx, func(client pb.HostServiceClient) error {
		if marker != "" {
			_, err := client.WriteFile(ctx, &pb.WriteFileRequest{
				Path:    marker,
				Content: []byte("post-init"),
				Mode:    0o644,
			})
			if err != nil {
				return err
			}
		}
		if stackName != "" {
			_, err := client.SetStackMetadata(ctx, &pb.SetStackRequest{
				Path: "stack",
				Metadata: &pb.StackMetadata{
					Name: stackName,
				},
				Merge: true,
			})
			if err != nil {
				return err
			}
		}
		return nil
	})
}

type grpcGenerateService struct {
	pb.UnimplementedGenerateServiceServer
}

func (s *grpcGenerateService) Generate(req *pb.GenerateRequest, stream pb.GenerateService_GenerateServer) error {
	if req == nil {
		return fmt.Errorf("generate request is required")
	}
	if err := stream.Send(&pb.GenerateOutput{
		Output: &pb.GenerateOutput_Stdout{Stdout: []byte("grpc generate\n")},
	}); err != nil {
		return err
	}
	if err := stream.Send(&pb.GenerateOutput{
		Output: &pb.GenerateOutput_FileWrite{
			FileWrite: &pb.FileWrite{
				Path:    "generated.txt",
				Content: []byte("generated by grpc plugin"),
				Mode:    0o644,
			},
		},
	}); err != nil {
		return err
	}
	return stream.Send(&pb.GenerateOutput{
		Output: &pb.GenerateOutput_ExitCode{ExitCode: 0},
	})
}

func withHostClient(ctx context.Context, fn func(pb.HostServiceClient) error) error {
	addr := os.Getenv(plugingrpc.HostAddrEnv)
	if addr == "" {
		return nil
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return err
	}
	defer func() {
		_ = conn.Close()
	}()
	client := pb.NewHostServiceClient(conn)
	return fn(client)
}
