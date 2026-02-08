// Copyright 2025 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package grpc

import (
	"context"
	"io"
	"net"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/terramate-io/terramate/config"
	pb "github.com/terramate-io/terramate/plugin/proto/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
)

func TestHostServiceStackMetadataRoundTrip(t *testing.T) {
	rootDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(rootDir, "terramate.tm.hcl"), []byte("terramate {}"), 0o644); err != nil {
		t.Fatalf("write root config: %v", err)
	}
	stackDir := filepath.Join(rootDir, "stack")
	if err := os.MkdirAll(stackDir, 0o755); err != nil {
		t.Fatalf("mkdir stack: %v", err)
	}
	if err := os.WriteFile(filepath.Join(stackDir, "stack.tm.hcl"), []byte("stack {}"), 0o644); err != nil {
		t.Fatalf("write stack config: %v", err)
	}

	root, err := config.LoadRoot(rootDir, false)
	if err != nil {
		t.Fatalf("load root: %v", err)
	}
	service := NewHostService(root, filepath.Join(rootDir, ".terramate"))

	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	server := grpc.NewServer()
	pb.RegisterHostServiceServer(server, service)
	go func() {
		_ = server.Serve(lis)
	}()
	defer server.Stop()

	conn, err := grpc.NewClient(lis.Addr().String(), grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer func() {
		_ = conn.Close()
	}()
	client := pb.NewHostServiceClient(conn)

	ctx := context.Background()
	_, err = client.SetStackMetadata(ctx, &pb.SetStackRequest{
		Path: "stack",
		Metadata: &pb.StackMetadata{
			Name:        "demo",
			Description: "desc",
			Tags:        []string{"one", "two"},
		},
		Merge: true,
	})
	if err != nil {
		t.Fatalf("set stack metadata: %v", err)
	}
	meta, err := client.GetStackMetadata(ctx, &pb.GetStackRequest{Path: "stack"})
	if err != nil {
		t.Fatalf("get stack metadata: %v", err)
	}
	if meta.Name != "demo" || meta.Description != "desc" || len(meta.Tags) != 2 {
		t.Fatalf("unexpected metadata: %+v", meta)
	}

	_, err = client.WriteFile(ctx, &pb.WriteFileRequest{
		Path:    filepath.Join(rootDir, "test.txt"),
		Content: []byte("hello"),
		Mode:    0o644,
	})
	if err != nil {
		t.Fatalf("write file: %v", err)
	}
	content, err := client.ReadFile(ctx, &pb.ReadFileRequest{Path: filepath.Join(rootDir, "test.txt")})
	if err != nil {
		t.Fatalf("read file: %v", err)
	}
	if string(content.Content) != "hello" {
		t.Fatalf("unexpected file content: %s", string(content.Content))
	}
}

func TestHostServiceWalkDirAndConfigTree(t *testing.T) {
	rootDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(rootDir, "terramate.tm.hcl"), []byte("terramate {}"), 0o644); err != nil {
		t.Fatalf("write root config: %v", err)
	}
	stackDir := filepath.Join(rootDir, "stack")
	if err := os.MkdirAll(stackDir, 0o755); err != nil {
		t.Fatalf("mkdir stack: %v", err)
	}
	if err := os.WriteFile(filepath.Join(stackDir, "stack.tm.hcl"), []byte("stack {}"), 0o644); err != nil {
		t.Fatalf("write stack config: %v", err)
	}
	if err := os.WriteFile(filepath.Join(rootDir, "README.md"), []byte("root"), 0o644); err != nil {
		t.Fatalf("write readme: %v", err)
	}

	root, err := config.LoadRoot(rootDir, false)
	if err != nil {
		t.Fatalf("load root: %v", err)
	}
	service := NewHostService(root, filepath.Join(rootDir, ".terramate"))

	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	server := grpc.NewServer()
	pb.RegisterHostServiceServer(server, service)
	go func() {
		_ = server.Serve(lis)
	}()
	defer server.Stop()

	conn, err := grpc.NewClient(lis.Addr().String(), grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer func() {
		_ = conn.Close()
	}()
	client := pb.NewHostServiceClient(conn)

	ctx := context.Background()
	stream, err := client.WalkDir(ctx, &pb.WalkDirRequest{Root: ".", Pattern: "*.md"})
	if err != nil {
		t.Fatalf("walk dir: %v", err)
	}
	var entries []string
	for {
		entry, recvErr := stream.Recv()
		if recvErr == io.EOF {
			break
		}
		if recvErr != nil {
			t.Fatalf("walk recv: %v", recvErr)
		}
		entries = append(entries, entry.Path)
	}
	if len(entries) != 1 || entries[0] != "README.md" {
		t.Fatalf("unexpected entries: %#v", entries)
	}

	rootNode, err := client.GetConfigTree(ctx, &pb.GetConfigTreeRequest{Path: "/"})
	if err != nil {
		t.Fatalf("get config tree: %v", err)
	}
	if rootNode.IsStack {
		t.Fatalf("expected root not to be a stack")
	}
	stackNode, err := client.GetConfigTree(ctx, &pb.GetConfigTreeRequest{Path: "stack"})
	if err != nil {
		t.Fatalf("get stack node: %v", err)
	}
	if !stackNode.IsStack {
		t.Fatalf("expected stack to be a stack node")
	}
}

func TestHostServiceInvalidPaths(t *testing.T) {
	rootDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(rootDir, "terramate.tm.hcl"), []byte("terramate {}"), 0o644); err != nil {
		t.Fatalf("write root config: %v", err)
	}
	root, err := config.LoadRoot(rootDir, false)
	if err != nil {
		t.Fatalf("load root: %v", err)
	}
	service := NewHostService(root, filepath.Join(rootDir, ".terramate"))

	_, err = service.ReadFile(context.Background(), &pb.ReadFileRequest{Path: ""})
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("expected invalid argument, got %v", err)
	}

	outside := filepath.Join(t.TempDir(), "outside.txt")
	_, err = service.ReadFile(context.Background(), &pb.ReadFileRequest{Path: outside})
	if status.Code(err) != codes.PermissionDenied {
		t.Fatalf("expected permission denied, got %v", err)
	}
}

func TestHostServiceUnavailableWithoutRoot(t *testing.T) {
	service := NewHostService(nil, "")

	_, err := service.GetConfigTree(context.Background(), &pb.GetConfigTreeRequest{Path: "/"})
	if status.Code(err) != codes.Unavailable {
		t.Fatalf("expected unavailable, got %v", err)
	}

	_, err = service.GetStackMetadata(context.Background(), &pb.GetStackRequest{Path: "stack"})
	if status.Code(err) != codes.Unavailable {
		t.Fatalf("expected unavailable, got %v", err)
	}

	_, err = service.SetStackMetadata(context.Background(), &pb.SetStackRequest{Path: "stack"})
	if status.Code(err) != codes.Unavailable {
		t.Fatalf("expected unavailable, got %v", err)
	}
}

func TestHostServiceConcurrentRootAccess(t *testing.T) {
	rootDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(rootDir, "terramate.tm.hcl"), []byte("terramate {}"), 0o644); err != nil {
		t.Fatalf("write root config: %v", err)
	}
	root, err := config.LoadRoot(rootDir, false)
	if err != nil {
		t.Fatalf("load root: %v", err)
	}
	service := NewHostService(root, filepath.Join(rootDir, ".terramate"))

	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _ = service.GetRootDir(context.Background(), &pb.Empty{})
		}()
	}
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			service.SetRoot(root)
		}()
	}
	wg.Wait()
}
