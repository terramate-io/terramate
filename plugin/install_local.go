// Copyright 2025 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package plugin

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/go-plugin"
	"google.golang.org/grpc"
)

// InstallFromLocal installs a plugin from a local directory.
func InstallFromLocal(_ context.Context, name string, opts InstallOptions) (Manifest, error) {
	if opts.Source == "" {
		return Manifest{}, fmt.Errorf("local source is required")
	}
	info, err := os.Stat(opts.Source)
	if err != nil {
		return Manifest{}, err
	}
	if !info.IsDir() {
		return Manifest{}, fmt.Errorf("source must be a directory")
	}
	pluginDir := PluginDir(opts.UserTerramateDir, name)
	if err := os.MkdirAll(pluginDir, 0o700); err != nil {
		return Manifest{}, err
	}
	binaries, err := detectLocalBinaries(opts.Source, pluginDir, name)
	if err != nil {
		return Manifest{}, err
	}
	protocol, err := detectLocalProtocol(pluginDir, binaries)
	if err != nil {
		return Manifest{}, err
	}
	m := Manifest{
		Name:     name,
		Version:  "local",
		Type:     TypeGRPC,
		Protocol: protocol,
		Binaries: binaries,
	}
	if err := SaveManifest(pluginDir, m); err != nil {
		return Manifest{}, err
	}
	return m, nil
}

func detectLocalBinaries(sourceDir, pluginDir, name string) (map[BinaryKind]Binary, error) {
	suffix := exeSuffix()
	cliPath := filepath.Join(sourceDir, "terramate"+suffix)
	lsPath := filepath.Join(sourceDir, "terramate-ls"+suffix)
	pluginCLIPath := filepath.Join(sourceDir, "terramate-"+name+suffix)
	pluginLSPath := filepath.Join(sourceDir, "terramate-"+name+"-ls"+suffix)
	binaries := map[BinaryKind]Binary{}

	if _, err := os.Stat(cliPath); err == nil {
		target := filepath.Join(pluginDir, filepath.Base(cliPath))
		if err := copyFile(cliPath, target, 0o700); err != nil {
			return nil, err
		}
		binaries[BinaryCLI] = Binary{Path: filepath.Base(target)}
	}

	if _, err := os.Stat(lsPath); err == nil {
		target := filepath.Join(pluginDir, filepath.Base(lsPath))
		if err := copyFile(lsPath, target, 0o700); err != nil {
			return nil, err
		}
		binaries[BinaryLS] = Binary{Path: filepath.Base(target)}
	}

	if _, ok := binaries[BinaryCLI]; !ok {
		if _, err := os.Stat(pluginCLIPath); err == nil {
			target := filepath.Join(pluginDir, filepath.Base(pluginCLIPath))
			if err := copyFile(pluginCLIPath, target, 0o700); err != nil {
				return nil, err
			}
			binaries[BinaryCLI] = Binary{Path: filepath.Base(target)}
		}
	}

	if _, ok := binaries[BinaryLS]; !ok {
		if _, err := os.Stat(pluginLSPath); err == nil {
			target := filepath.Join(pluginDir, filepath.Base(pluginLSPath))
			if err := copyFile(pluginLSPath, target, 0o700); err != nil {
				return nil, err
			}
			binaries[BinaryLS] = Binary{Path: filepath.Base(target)}
		}
	}

	if _, ok := binaries[BinaryCLI]; !ok {
		if err := detectGRPCBinary(sourceDir, pluginDir, binaries); err != nil {
			return nil, err
		}
	}

	if _, ok := binaries[BinaryCLI]; !ok {
		return nil, fmt.Errorf("source directory does not contain terramate binary")
	}
	return binaries, nil
}

func detectGRPCBinary(sourceDir, pluginDir string, binaries map[BinaryKind]Binary) error {
	entries, err := os.ReadDir(sourceDir)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !isExecutableCandidate(entry, name) {
			continue
		}
		path := filepath.Join(sourceDir, name)
		if !supportsGRPCPlugin(path) {
			continue
		}
		target := filepath.Join(pluginDir, filepath.Base(path))
		if err := copyFile(path, target, 0o700); err != nil {
			return err
		}
		binaries[BinaryCLI] = Binary{Path: filepath.Base(target)}
		return nil
	}
	return nil
}

func isExecutableCandidate(entry os.DirEntry, name string) bool {
	if runtime.GOOS == "windows" {
		return filepath.Ext(name) == ".exe"
	}
	info, err := entry.Info()
	if err != nil {
		return false
	}
	return info.Mode()&0o111 != 0
}

const (
	grpcPluginName           = "terramate-grpc"
	handshakeProtocolVersion = 1
	magicCookieKey           = "TM_PLUGIN_MAGIC_COOKIE"
	magicCookieValue         = "terramate"
)

type grpcProbePlugin struct {
	plugin.NetRPCUnsupportedPlugin
}

func (p *grpcProbePlugin) GRPCServer(*plugin.GRPCBroker, *grpc.Server) error {
	return nil
}

func (p *grpcProbePlugin) GRPCClient(context.Context, *plugin.GRPCBroker, *grpc.ClientConn) (interface{}, error) {
	return struct{}{}, nil
}

func detectLocalProtocol(pluginDir string, binaries map[BinaryKind]Binary) (Protocol, error) {
	cli, ok := binaries[BinaryCLI]
	if !ok || cli.Path == "" {
		return "", fmt.Errorf("plugin CLI binary not found")
	}
	path := filepath.Join(pluginDir, cli.Path)
	if supportsGRPCPlugin(path) {
		return ProtocolGRPC, nil
	}
	return "", fmt.Errorf("plugin binary does not support gRPC protocol")
}

func supportsGRPCPlugin(binaryPath string) bool {
	cmd := exec.Command(binaryPath)
	cmd.Env = append(os.Environ(), fmt.Sprintf("%s=%s", magicCookieKey, magicCookieValue))
	cfg := &plugin.ClientConfig{
		HandshakeConfig: plugin.HandshakeConfig{
			ProtocolVersion:  handshakeProtocolVersion,
			MagicCookieKey:   magicCookieKey,
			MagicCookieValue: magicCookieValue,
		},
		Plugins: map[string]plugin.Plugin{
			grpcPluginName: &grpcProbePlugin{},
		},
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
		return false
	}
	_, err = rpcClient.Dispense(grpcPluginName)
	client.Kill()
	return err == nil
}

func copyFile(src, dest string, mode os.FileMode) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer func() {
		_ = in.Close()
	}()

	tmp := dest + ".tmp"
	out, err := os.OpenFile(tmp, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		_ = out.Close()
		return err
	}
	if err := out.Close(); err != nil {
		return err
	}
	return os.Rename(tmp, dest)
}

func exeSuffix() string {
	if runtime.GOOS == "windows" {
		return ".exe"
	}
	return ""
}
