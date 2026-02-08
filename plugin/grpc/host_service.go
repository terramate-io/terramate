// Copyright 2025 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package grpc

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/terramate-io/terramate/config"
	"github.com/terramate-io/terramate/hcl"
	pb "github.com/terramate-io/terramate/plugin/proto/v1"
	"github.com/terramate-io/terramate/project"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// HostService provides filesystem and config access to plugins.
type HostService struct {
	pb.UnimplementedHostServiceServer
	mu               sync.RWMutex
	root             *config.Root
	userTerramateDir string
	rootDir          string
}

// NewHostService constructs a HostService for the given root.
func NewHostService(root *config.Root, userTerramateDir string) *HostService {
	rootDir := ""
	if root != nil {
		rootDir = root.HostDir()
	}
	return &HostService{
		root:             root,
		userTerramateDir: userTerramateDir,
		rootDir:          rootDir,
	}
}

// SetRoot updates the root configuration. This is thread-safe and can be called
// after the service has been started to update the root once it becomes available.
func (s *HostService) SetRoot(root *config.Root) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.root = root
	if root != nil {
		s.rootDir = root.HostDir()
	} else {
		s.rootDir = ""
	}
}

// GetRootDir returns the host root directory path.
func (s *HostService) GetRootDir(context.Context, *pb.Empty) (*pb.StringValue, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return &pb.StringValue{Value: s.rootDir}, nil
}

// GetUserTerramateDir returns the configured user terramate directory.
func (s *HostService) GetUserTerramateDir(context.Context, *pb.Empty) (*pb.StringValue, error) {
	return &pb.StringValue{Value: s.userTerramateDir}, nil
}

// ReadFile returns the contents of a host file.
func (s *HostService) ReadFile(_ context.Context, req *pb.ReadFileRequest) (*pb.ReadFileResponse, error) {
	s.mu.RLock()
	path, err := s.resolvePath(req.Path)
	s.mu.RUnlock()
	if err != nil {
		return nil, err
	}
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return &pb.ReadFileResponse{Content: content}, nil
}

// WriteFile writes a file on the host filesystem.
func (s *HostService) WriteFile(_ context.Context, req *pb.WriteFileRequest) (*pb.Empty, error) {
	s.mu.RLock()
	path, err := s.resolvePath(req.Path)
	s.mu.RUnlock()
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	mode := os.FileMode(req.Mode)
	if mode == 0 {
		mode = 0o644
	}
	if err := os.WriteFile(path, req.Content, mode); err != nil {
		return nil, err
	}
	return &pb.Empty{}, nil
}

// WalkDir streams directory entries for the requested root.
func (s *HostService) WalkDir(req *pb.WalkDirRequest, stream pb.HostService_WalkDirServer) error {
	s.mu.RLock()
	root, err := s.resolvePath(req.Root)
	s.mu.RUnlock()
	if err != nil {
		return err
	}
	pattern := strings.TrimSpace(req.Pattern)
	return filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if pattern != "" {
			ok, matchErr := filepath.Match(pattern, filepath.Base(path))
			if matchErr != nil {
				return matchErr
			}
			if !ok {
				return nil
			}
		}
		rel, relErr := filepath.Rel(root, path)
		if relErr != nil {
			rel = path
		}
		return stream.Send(&pb.DirEntry{
			Path:  filepath.ToSlash(rel),
			IsDir: entry.IsDir(),
		})
	})
}

// GetConfigTree returns the config tree node for a given path.
func (s *HostService) GetConfigTree(_ context.Context, req *pb.GetConfigTreeRequest) (*pb.ConfigTreeNode, error) {
	s.mu.RLock()
	if s.root == nil {
		s.mu.RUnlock()
		return nil, status.Error(codes.Unavailable, "configuration is not loaded")
	}
	path, err := s.toProjectPath(req.Path)
	if err != nil {
		s.mu.RUnlock()
		return nil, err
	}
	node, found := s.root.Lookup(path)
	s.mu.RUnlock()
	if !found {
		return nil, status.Error(codes.NotFound, "configuration path not found")
	}
	return &pb.ConfigTreeNode{
		Dir:     node.Dir().String(),
		IsStack: node.IsStack(),
		Stack:   stackMetadata(node),
	}, nil
}

// GetStackMetadata returns metadata for a stack.
func (s *HostService) GetStackMetadata(_ context.Context, req *pb.GetStackRequest) (*pb.StackMetadata, error) {
	s.mu.RLock()
	if s.root == nil {
		s.mu.RUnlock()
		return nil, status.Error(codes.Unavailable, "configuration is not loaded")
	}
	path, err := s.toProjectPath(req.Path)
	if err != nil {
		s.mu.RUnlock()
		return nil, err
	}
	node, found := s.root.Lookup(path)
	s.mu.RUnlock()
	if !found {
		return nil, status.Error(codes.NotFound, "stack path not found")
	}
	return stackMetadata(node), nil
}

// SetStackMetadata updates stack metadata.
func (s *HostService) SetStackMetadata(_ context.Context, req *pb.SetStackRequest) (*pb.Empty, error) {
	s.mu.RLock()
	if s.root == nil {
		s.mu.RUnlock()
		return nil, status.Error(codes.Unavailable, "configuration is not loaded")
	}
	path, err := s.toProjectPath(req.Path)
	if err != nil {
		s.mu.RUnlock()
		return nil, err
	}
	node, found := s.root.Lookup(path)
	s.mu.RUnlock()
	if !found {
		return nil, status.Error(codes.NotFound, "stack path not found")
	}
	if node.Node.Stack == nil {
		node.Node.Stack = &hclStack{}
	}
	if req.Metadata == nil {
		return &pb.Empty{}, nil
	}
	applyStackMetadata(node.Node.Stack, req.Metadata, req.Merge)
	return &pb.Empty{}, nil
}

func (s *HostService) resolvePath(path string) (string, error) {
	if path == "" {
		return "", status.Error(codes.InvalidArgument, "path is required")
	}
	if filepath.IsAbs(path) {
		if s.rootDir != "" && !strings.HasPrefix(filepath.Clean(path), filepath.Clean(s.rootDir)+string(filepath.Separator)) && path != s.rootDir {
			return "", status.Error(codes.PermissionDenied, "path outside root directory")
		}
		return filepath.Clean(path), nil
	}
	if s.rootDir == "" {
		return "", status.Error(codes.InvalidArgument, "root directory is unknown")
	}
	return filepath.Join(s.rootDir, path), nil
}

func (s *HostService) toProjectPath(path string) (project.Path, error) {
	if path == "" || path == "/" {
		return project.NewPath("/"), nil
	}
	if filepath.IsAbs(path) {
		if s.rootDir != "" {
			cleanPath := filepath.Clean(path)
			cleanRoot := filepath.Clean(s.rootDir)
			if strings.HasPrefix(cleanPath, cleanRoot+string(filepath.Separator)) || cleanPath == cleanRoot {
				return project.PrjAbsPath(s.rootDir, path), nil
			}
		}
		if strings.HasPrefix(path, "/") {
			return project.NewPath(path), nil
		}
		return project.Path{}, status.Error(codes.InvalidArgument, "invalid absolute path")
	}
	if strings.HasPrefix(path, "/") {
		return project.NewPath(path), nil
	}
	return project.NewPath("/" + filepath.ToSlash(path)), nil
}

type hclStack = hcl.Stack

func stackMetadata(node *config.Tree) *pb.StackMetadata {
	if node == nil || node.Node.Stack == nil {
		return &pb.StackMetadata{}
	}
	st := node.Node.Stack
	return &pb.StackMetadata{
		Name:        st.Name,
		Description: st.Description,
		Tags:        append([]string{}, st.Tags...),
		After:       append([]string{}, st.After...),
		Before:      append([]string{}, st.Before...),
		Wants:       append([]string{}, st.Wants...),
		WantedBy:    append([]string{}, st.WantedBy...),
		Watch:       append([]string{}, st.Watch...),
	}
}

func applyStackMetadata(stack *hclStack, meta *pb.StackMetadata, merge bool) {
	if stack == nil || meta == nil {
		return
	}
	if !merge || stack.Name == "" {
		stack.Name = meta.Name
	}
	if !merge || stack.Description == "" {
		stack.Description = meta.Description
	}
	if !merge || len(stack.Tags) == 0 {
		stack.Tags = append([]string{}, meta.Tags...)
	} else if len(meta.Tags) > 0 {
		stack.Tags = append(stack.Tags, meta.Tags...)
	}
	if !merge || len(stack.After) == 0 {
		stack.After = append([]string{}, meta.After...)
	} else if len(meta.After) > 0 {
		stack.After = append(stack.After, meta.After...)
	}
	if !merge || len(stack.Before) == 0 {
		stack.Before = append([]string{}, meta.Before...)
	} else if len(meta.Before) > 0 {
		stack.Before = append(stack.Before, meta.Before...)
	}
	if !merge || len(stack.Wants) == 0 {
		stack.Wants = append([]string{}, meta.Wants...)
	} else if len(meta.Wants) > 0 {
		stack.Wants = append(stack.Wants, meta.Wants...)
	}
	if !merge || len(stack.WantedBy) == 0 {
		stack.WantedBy = append([]string{}, meta.WantedBy...)
	} else if len(meta.WantedBy) > 0 {
		stack.WantedBy = append(stack.WantedBy, meta.WantedBy...)
	}
	if !merge || len(stack.Watch) == 0 {
		stack.Watch = append([]string{}, meta.Watch...)
	} else if len(meta.Watch) > 0 {
		stack.Watch = append(stack.Watch, meta.Watch...)
	}
}
