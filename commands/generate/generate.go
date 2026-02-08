// Copyright 2025 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

// Package generate provides the generate command.
package generate

import (
	"context"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"time"

	"github.com/rs/zerolog/log"
	"github.com/terramate-io/terramate/commands"
	"github.com/terramate-io/terramate/di"
	"github.com/terramate-io/terramate/engine"
	"github.com/terramate-io/terramate/errors"
	"github.com/terramate-io/terramate/event"
	"github.com/terramate-io/terramate/exit"
	"github.com/terramate-io/terramate/generate"
	"github.com/terramate-io/terramate/hcl"
	"github.com/terramate-io/terramate/modvendor/download"
	plugingrpc "github.com/terramate-io/terramate/plugin/grpc"
	pb "github.com/terramate-io/terramate/plugin/proto/v1"
	"github.com/terramate-io/terramate/printer"
	"github.com/terramate-io/terramate/project"
)

const defaultVendorDir = "/modules"

// Spec is the command specification for the generate command.
type Spec struct {
	DetailedExitCode bool
	Parallel         int

	MinimalReport bool
	PrintReport   bool

	workingDir string
	engine     *engine.Engine
	printers   printer.Printers
}

// Name returns the name of the command.
func (s *Spec) Name() string { return "generate" }

// Requirements returns the requirements of the command.
func (s *Spec) Requirements(context.Context, commands.CLI) any { return commands.RequireEngine() }

// Exec executes the generate command.
func (s *Spec) Exec(ctx context.Context, cli commands.CLI) error {
	s.workingDir = cli.WorkingDir()
	s.engine = cli.Engine()
	s.printers = cli.Printers()

	logger := log.With().
		Str("action", "commands/generate").
		Logger()

	usedOverride, err := s.execGenerateOverride(ctx, cli)
	if err != nil {
		return err
	}
	if usedOverride {
		return nil
	}

	vendorProgressEvents := download.NewEventStream()

	progressHandlerDone := make(chan struct{})

	go func() {
		for event := range vendorProgressEvents {
			s.printers.Stdout.Println(fmt.Sprintf("vendor: %s %s at %s",
				event.Message, event.Module.Raw, event.TargetDir))

			logger.Info().
				Str("module", event.Module.Raw).
				Stringer("vendorDir", event.TargetDir).
				Msg(event.Message)
		}

		close(progressHandlerDone)
	}()

	cfg := s.engine.Config()
	rootdir := cfg.HostDir()
	vendorRequestEvents := make(chan event.VendorRequest)
	vendorReports := download.HandleVendorRequests(
		rootdir,
		vendorRequestEvents,
		vendorProgressEvents,
	)

	mergedVendorReport := download.MergeVendorReports(vendorReports)

	logger.Trace().Msg("generating code")

	cwd := project.PrjAbsPath(rootdir, s.workingDir)
	vdir, err := s.vendorDir()
	if err != nil {
		return err
	}

	generateAPI, err := di.Get[generate.API](ctx)
	if err != nil {
		return err
	}

	report := generateAPI.Do(cfg, cwd, s.Parallel, vdir, vendorRequestEvents)

	logger.Trace().Msg("code generation finished, waiting for vendor requests to be handled")

	close(vendorRequestEvents)

	logger.Trace().Msg("waiting for vendor report merging")

	vendorReport := <-mergedVendorReport

	log.Trace().Msg("waiting for all progress events")

	close(vendorProgressEvents)
	<-progressHandlerDone

	log.Trace().Msg("all handlers stopped, generating final report")

	if s.PrintReport || report.HasFailures() {
		if s.MinimalReport {
			if minimalReport := report.Minimal(); minimalReport != "" {
				s.printers.Stdout.Println(minimalReport)
			}
		} else {
			s.printers.Stdout.Println(report.Full())
		}
	}

	vendorReport.RemoveIgnoredByKind(download.ErrAlreadyVendored)

	if !vendorReport.IsEmpty() {
		s.printers.Stdout.Println(vendorReport.String())
	}

	if s.DetailedExitCode {
		if len(report.Successes) > 0 || !vendorReport.IsEmpty() {
			return errors.E(exit.Changed)
		}
	}

	if report.HasFailures() || vendorReport.HasFailures() {
		return errors.E(exit.Failed)
	}
	return nil
}

func (s *Spec) execGenerateOverride(ctx context.Context, cli commands.CLI) (bool, error) {
	if val := os.Getenv("TM_DISABLE_GRPC_PLUGINS"); val != "" && val != "0" && val != "false" {
		return false, nil
	}
	userDir := cli.Config().UserTerramateDir
	if userDir == "" {
		return false, nil
	}
	logger := log.With().Str("action", "commands/generate/override").Logger()
	const generateTimeout = 15 * time.Minute

	installed, err := plugingrpc.DiscoverInstalled(userDir)
	if err != nil {
		return false, err
	}
	for _, plg := range installed {
		pluginCtx, cancel := context.WithTimeout(ctx, generateTimeout)
		client, err := plugingrpc.NewHostClient(plg.BinaryPath)
		if err != nil {
			logger.Error().Err(err).Str("plugin", plg.Manifest.Name).Msg("failed to start plugin")
			cancel()
			continue
		}
		grpcClient := client.Client()
		caps, err := grpcClient.PluginService.GetCapabilities(pluginCtx, &pb.Empty{})
		if err != nil {
			client.Kill()
			logger.Error().Err(err).Str("plugin", plg.Manifest.Name).Msg("failed to fetch plugin capabilities")
			cancel()
			continue
		}
		if caps == nil || !caps.HasGenerateOverride {
			client.Kill()
			cancel()
			continue
		}
		req := &pb.GenerateRequest{
			RootDir:          s.engine.Config().HostDir(),
			DetailedExitCode: s.DetailedExitCode,
		}
		stream, err := grpcClient.GenerateService.Generate(pluginCtx, req)
		if err != nil {
			client.Kill()
			logger.Error().Err(err).Str("plugin", plg.Manifest.Name).Msg("generate override failed to start")
			cancel()
			continue
		}
		exitCode, err := s.consumeGenerateStream(stream, req.RootDir)
		client.Kill()
		cancel()
		if err != nil {
			logger.Error().Err(err).Str("plugin", plg.Manifest.Name).Msg("generate override failed")
			continue
		}
		if exitCode != 0 {
			return true, errors.E(exit.Status(exitCode))
		}
		return true, nil
	}
	return false, nil
}

func (s *Spec) consumeGenerateStream(stream pb.GenerateService_GenerateClient, rootDir string) (int32, error) {
	var exitCode int32
	for {
		output, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			return exitCode, err
		}
		switch msg := output.Output.(type) {
		case *pb.GenerateOutput_Stdout:
			if len(msg.Stdout) > 0 {
				_, _ = s.printers.Stdout.Write(msg.Stdout)
			}
		case *pb.GenerateOutput_Stderr:
			if len(msg.Stderr) > 0 {
				_, _ = s.printers.Stderr.Write(msg.Stderr)
			}
		case *pb.GenerateOutput_FileWrite:
			if err := s.writeGenerateFile(rootDir, msg.FileWrite); err != nil {
				return exitCode, err
			}
		case *pb.GenerateOutput_ExitCode:
			exitCode = msg.ExitCode
		}
	}
	return exitCode, nil
}

func (s *Spec) writeGenerateFile(rootDir string, file *pb.FileWrite) error {
	if file == nil || file.Path == "" {
		return nil
	}
	target := file.Path
	if !filepath.IsAbs(target) {
		target = filepath.Join(rootDir, target)
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return err
	}
	mode := os.FileMode(file.Mode)
	if mode == 0 {
		mode = 0o644
	}
	return os.WriteFile(target, file.Content, mode)
}

func (s *Spec) vendorDir() (project.Path, error) {
	checkVendorDir := func(dir string) (project.Path, error) {
		if !path.IsAbs(dir) {
			return project.Path{}, errors.E("vendorDir %s defined is not an absolute path", dir)
		}
		return project.NewPath(dir), nil
	}

	rootdir := s.engine.Config().HostDir()
	dotTerramate := filepath.Join(rootdir, ".terramate")
	dotTerramateInfo, err := os.Stat(dotTerramate)

	if err == nil && dotTerramateInfo.IsDir() {
		cfg, err := hcl.ParseDir(rootdir, dotTerramate, s.engine.HCLOptions()...)
		if err != nil {
			return project.Path{}, errors.E(err, "parsing vendor dir configuration on .terramate")
		}
		if hasVendorDirConfig(cfg) {
			return checkVendorDir(cfg.Vendor.Dir)
		}
	}
	hclcfg := s.engine.Config().Tree().Node
	if hasVendorDirConfig(&hclcfg) {
		return checkVendorDir(hclcfg.Vendor.Dir)
	}
	return project.NewPath(defaultVendorDir), nil
}

func hasVendorDirConfig(cfg *hcl.Config) bool {
	return cfg.Vendor != nil && cfg.Vendor.Dir != ""
}
