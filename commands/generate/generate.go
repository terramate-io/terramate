package generate

import (
	"context"
	"fmt"
	"os"
	"path"
	"path/filepath"

	"github.com/rs/zerolog/log"
	"github.com/terramate-io/terramate/engine"
	"github.com/terramate-io/terramate/errors"
	"github.com/terramate-io/terramate/event"
	"github.com/terramate-io/terramate/exit"
	"github.com/terramate-io/terramate/generate"
	"github.com/terramate-io/terramate/hcl"
	"github.com/terramate-io/terramate/modvendor/download"
	"github.com/terramate-io/terramate/printer"
	"github.com/terramate-io/terramate/project"
)

const defaultVendorDir = "/modules"

type Spec struct {
	Engine           *engine.Engine
	WorkingDir       string
	DetailedExitCode bool
	Parallel         int
	Printers         printer.Printers
	MinimalReport    bool
}

func (s *Spec) Name() string { return "generate" }

func (s *Spec) Exec(ctx context.Context) error {
	logger := log.With().
		Str("action", "commands/generate").
		Logger()

	vendorProgressEvents := download.NewEventStream()

	progressHandlerDone := make(chan struct{})

	go func() {
		for event := range vendorProgressEvents {
			s.Printers.Stdout.Println(fmt.Sprintf("vendor: %s %s at %s",
				event.Message, event.Module.Raw, event.TargetDir))

			logger.Info().
				Str("module", event.Module.Raw).
				Stringer("vendorDir", event.TargetDir).
				Msg(event.Message)
		}

		close(progressHandlerDone)
	}()

	cfg := s.Engine.Config()
	rootdir := cfg.HostDir()
	vendorRequestEvents := make(chan event.VendorRequest)
	vendorReports := download.HandleVendorRequests(
		rootdir,
		vendorRequestEvents,
		vendorProgressEvents,
	)

	mergedVendorReport := download.MergeVendorReports(vendorReports)

	logger.Trace().Msg("generating code")

	cwd := project.PrjAbsPath(rootdir, s.WorkingDir)
	vdir, err := s.vendorDir()
	if err != nil {
		return err
	}
	report := generate.Do(cfg, cwd, s.Parallel, vdir, vendorRequestEvents)

	logger.Trace().Msg("code generation finished, waiting for vendor requests to be handled")

	close(vendorRequestEvents)

	logger.Trace().Msg("waiting for vendor report merging")

	vendorReport := <-mergedVendorReport

	log.Trace().Msg("waiting for all progress events")

	close(vendorProgressEvents)
	<-progressHandlerDone

	log.Trace().Msg("all handlers stopped, generating final report")

	if s.MinimalReport {
		s.Printers.Stdout.Println(report.Minimal())
	} else {
		s.Printers.Stdout.Println(report.Full())
	}

	vendorReport.RemoveIgnoredByKind(download.ErrAlreadyVendored)

	err = nil
	if !vendorReport.IsEmpty() {
		s.Printers.Stdout.Println(vendorReport.String())
	}

	if s.DetailedExitCode {
		if len(report.Successes) > 0 || !vendorReport.IsEmpty() {
			err = errors.E(exit.Changed, "code generated with changes")
		}
	}

	if report.HasFailures() || vendorReport.HasFailures() {
		err = errors.E(exit.Failed, "generation failed")
	}
	return err
}

func (s *Spec) vendorDir() (project.Path, error) {
	checkVendorDir := func(dir string) (project.Path, error) {
		if !path.IsAbs(dir) {
			return project.Path{}, errors.E("vendorDir %s defined is not an absolute path", dir)
		}
		return project.NewPath(dir), nil
	}

	rootdir := s.Engine.Config().HostDir()
	dotTerramate := filepath.Join(rootdir, ".terramate")
	dotTerramateInfo, err := os.Stat(dotTerramate)

	if err == nil && dotTerramateInfo.IsDir() {
		cfg, err := hcl.ParseDir(rootdir, filepath.Join(rootdir, ".terramate"))
		if err != nil {
			return project.Path{}, errors.E(err, "parsing vendor dir configuration on .terramate")
		}
		if hasVendorDirConfig(cfg) {
			return checkVendorDir(cfg.Vendor.Dir)
		}
	}
	hclcfg := s.Engine.Config().Tree().Node
	if hasVendorDirConfig(hclcfg) {
		return checkVendorDir(hclcfg.Vendor.Dir)
	}
	return project.NewPath(defaultVendorDir), nil
}

func hasVendorDirConfig(cfg hcl.Config) bool {
	return cfg.Vendor != nil && cfg.Vendor.Dir != ""
}
