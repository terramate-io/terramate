// Copyright 2023 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

// Package download is responsible for downloading vendored modules.
package download

import (
	"fmt"
	iofs "io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/hashicorp/hcl/v2/hclwrite"
	"github.com/rs/zerolog/log"
	"github.com/terramate-io/terramate/errors"
	"github.com/terramate-io/terramate/event"
	"github.com/terramate-io/terramate/fs"
	"github.com/terramate-io/terramate/git"
	"github.com/terramate-io/terramate/modvendor"
	"github.com/terramate-io/terramate/modvendor/manifest"
	"github.com/terramate-io/terramate/project"
	"github.com/terramate-io/terramate/tf"
	"github.com/zclconf/go-cty/cty"

	hhcl "github.com/hashicorp/hcl/v2"
)

const (
	// ErrAlreadyVendored indicates that a module is already vendored.
	ErrAlreadyVendored errors.Kind = "module is already vendored"

	// ErrUnsupportedModSrc indicates that the module source is not supported.
	ErrUnsupportedModSrc errors.Kind = "unsupported module source"

	// ErrDownloadMod indicates that an error occurred while trying to download a module.
	ErrDownloadMod errors.Kind = "downloading module"

	// ErrModRefEmpty indicates that a module source had no reference on it.
	ErrModRefEmpty errors.Kind = "module ref is empty"
)

type modinfo struct {
	source     string
	vendoredAt project.Path
	origin     string
	subdir     string
}

// Vendor will vendor the given module and its dependencies inside the provided
// root dir.
// The root dir must be an absolute path.
// The vendor dir must be an absolute path that will be considered as relative
// to the given rootdir.
//
// Vendored modules will be located at:
//
// - <rootdir>/<vendordir>/<Source.Path>/<Source.Ref>
//
// The whole path inside the vendor dir will be created if it not exists.
// Vendoring will not download any git submodules.
//
// The remote git module dependencies will also be vendored and each
// module.source declaration for those dependencies will be rewritten to
// reference them inside the vendor directory.
//
// An [EventStream] instance may be passed if the caller is interested on
// live events from what is happening inside the vendoring process. Passing
// a nil EventStream ignores all events.
// It is the caller responsibility to close the [EventStream] after the Vendor
// call returns.
//
// It returns a report of everything vendored and ignored (with a reason).
func Vendor(
	rootdir string,
	vendorDir project.Path,
	modsrc tf.Source,
	events ProgressEventStream,
) Report {
	return vendor(rootdir, vendorDir, modsrc, NewReport(vendorDir), nil, events)
}

// HandleVendorRequests starts a goroutine that will handle all vendor requests
// sent on vendorRequests, calling [Vendor] for each event and sending one report
// result for each event on the returned Report channel.
//
// It will read events from vendorRequests until it is closed.
// If progressEvents is nil it will not send any progress events, like [Vendor].
//
// When vendorRequests is closed it will close the returned [Report] channel,
// indicating that no more processing will be done.
func HandleVendorRequests(
	rootdir string,
	vendorRequests <-chan event.VendorRequest,
	progressEvents ProgressEventStream,
) <-chan Report {
	reportsStream := make(chan Report)
	go func() {
		logger := log.With().
			Str("action", "download.HandleVendorRequests()").
			Str("rootdir", rootdir).
			Logger()

		logger.Debug().Msg("starting vendor request handler")

		for vendorRequest := range vendorRequests {
			logger = logger.With().
				Str("modsrc", vendorRequest.Source.Raw).
				Stringer("vendorDir", vendorRequest.VendorDir).
				Logger()

			logger.Debug().Msgf("handling vendor request")

			report := Vendor(rootdir, vendorRequest.VendorDir, vendorRequest.Source, progressEvents)

			logger.Debug().Msgf("handled vendor request, sending report")

			reportsStream <- report

			logger.Debug().Msgf("report sent")
		}

		logger.Debug().Msg("stopping vendor request handler")

		close(reportsStream)
	}()
	return reportsStream
}

// MergeVendorReports will read all reports from the given reports channel, merge them
// and send the merged result on the returned channel and then close it.
// The returned channel always produce a single final report after the given
// reports channel is closed.
func MergeVendorReports(reports <-chan Report) <-chan Report {
	mergedReport := make(chan Report)
	go func() {
		logger := log.With().
			Str("action", "download.MergeVendorReports").
			Logger()

		var finalReport Report

		logger.Debug().Msg("starting to merge vendor reports")

		for report := range reports {
			logger.Debug().Msg("got vendor report, merging")

			if finalReport.IsEmpty() {
				finalReport = report
				continue
			}

			finalReport.merge(report)
		}

		logger.Debug().Msg("finished merging vendor reports, sending final report")

		if !finalReport.IsEmpty() {
			mergedReport <- finalReport
		}

		close(mergedReport)

		logger.Debug().Msg("sent final report, finished")
	}()
	return mergedReport
}

// VendorAll will vendor all dependencies of the tfdir into rootdir.
// It will scan all .tf files in the directory and vendor each module declaration
// containing the supported remote source URLs.
func VendorAll(
	rootdir string,
	vendorDir project.Path,
	tfdir string,
	events ProgressEventStream,
) Report {
	return vendorAll(rootdir, vendorDir, tfdir, NewReport(vendorDir), events)
}

func vendor(
	rootdir string,
	vendorDir project.Path,
	modsrc tf.Source,
	report Report,
	info *modinfo,
	events ProgressEventStream,
) Report {
	moddir, err := downloadVendor(rootdir, vendorDir, modsrc, events)
	if err != nil {
		if errors.IsKind(err, ErrAlreadyVendored) {
			report.addIgnored(modsrc.Raw, err)
			return report
		}

		errmsg := fmt.Sprintf("vendoring %q with ref %q",
			modsrc.URL, modsrc.Ref)

		if info != nil {
			errmsg += fmt.Sprintf(" found in %s", info.origin)
		}

		report.addIgnored(modsrc.Raw, errors.E(ErrDownloadMod, err, errmsg))
		return report
	}

	report.addVendored(modsrc)
	return vendorAll(rootdir, vendorDir, moddir, report, events)
}

// sourcesInfo represents information about module sources. It retains
// the original order that sources were appended, like an ordered map.
// both list and set always have the same modinfo inside, one is used
// for ordering dependent iteration, the other one for quick access of
// specific sources.
type sourcesInfo struct {
	list []*modinfo          // ordered list of sources
	set  map[string]*modinfo // set of sources
}

func newSourcesInfo() *sourcesInfo {
	return &sourcesInfo{
		set: make(map[string]*modinfo),
	}
}

func (s *sourcesInfo) append(source, path string) {
	if _, ok := s.set[source]; ok {
		return
	}
	info := &modinfo{
		source: source,
		origin: path,
	}
	s.set[source] = info
	s.list = append(s.list, info)
}

func (s *sourcesInfo) delete(source string) {
	for i, info := range s.list {
		if info.source == source {
			s.list = append(s.list[:i], s.list[i+1:]...)
			delete(s.set, source)
			return
		}
	}
}

func vendorAll(
	rootdir string,
	vendorDir project.Path,
	tfdir string,
	report Report,
	events ProgressEventStream,
) Report {
	sources := newSourcesInfo()
	originMap := map[string]struct{}{}
	errs := errors.L(report.Error)

	err := filepath.WalkDir(tfdir, func(path string, d iofs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if d.IsDir() && filepath.Base(path) == ".git" {
			return filepath.SkipDir
		}

		if !d.Type().IsRegular() || !strings.HasSuffix(path, ".tf") {
			return nil
		}

		modules, err := tf.ParseModules(path)
		if err != nil {
			errs.Append(err)
			return nil
		}

		for _, mod := range modules {
			if mod.IsLocal() || mod.Source == "" {
				continue
			}

			originMap[path] = struct{}{}

			sources.append(mod.Source, path)
		}
		return nil
	})

	errs.Append(err)

	for _, info := range sources.list {
		source := info.source
		modsrc, err := tf.ParseSource(source)
		if err != nil {
			report.addIgnored(source, err)
			sources.delete(source)
			continue
		}

		info.subdir = modsrc.Subdir

		targetVendorDir := modvendor.AbsVendorDir(rootdir, vendorDir, modsrc)
		st, err := os.Stat(targetVendorDir)
		if err == nil && st.IsDir() {
			info.vendoredAt = modvendor.TargetDir(vendorDir, modsrc)
			continue
		}

		report = vendor(rootdir, vendorDir, modsrc, report, info, events)
		if v, ok := report.Vendored[modvendor.TargetDir(vendorDir, modsrc)]; ok {
			info.vendoredAt = modvendor.TargetDir(vendorDir, modsrc)
			info.subdir = v.Source.Subdir

		}
	}

	files := []string{}
	for fname := range originMap {
		files = append(files, fname)
	}
	sort.Strings(files)
	errs.Append(patchFiles(rootdir, files, sources))
	report.Error = errs.AsError()
	return report
}

// downloadVendor will download the provided modsrc into the rootdir.
// If the project is already vendored an error of kind ErrAlreadyVendored will
// be returned, vendored projects are never updated.
// This function is not recursive, so dependencies won't have their dependencies
// vendored. See Vendor() for a recursive vendoring function.
func downloadVendor(
	rootdir string,
	vendorDir project.Path,
	modsrc tf.Source,
	events ProgressEventStream,
) (string, error) {
	if modsrc.Ref == "" {
		return "", errors.E(ErrModRefEmpty, "ref: %v", modsrc)
	}

	modVendorDir := modvendor.AbsVendorDir(rootdir, vendorDir, modsrc)
	if _, err := os.Stat(modVendorDir); err == nil {
		return "", errors.E(ErrAlreadyVendored, "dir %q exists", modVendorDir)
	}

	// We want an initial temporary dir outside of the Terramate project
	// to do the clone since some git setups will assume that any
	// git clone inside a repo is a submodule.
	clonedRepoDir, err := os.MkdirTemp("", ".tmvendor")
	if err != nil {
		return "", errors.E(err, "creating tmp clone dir")
	}
	defer func() {
		if err := os.RemoveAll(clonedRepoDir); err != nil {
			log.Warn().Err(err).
				Msg("deleting tmp clone dir")
		}
	}()

	// We want a temporary dir inside the project to where we are going to copy
	// the cloned module first. The idea is that if the copying fails we won't
	// leave any changes in the project vendor dir. The final step then will
	// be an atomic op using rename, which probably wont fail since the temp dir is
	// inside the project and the whole project is most likely on the same fs/device.
	tmTempDir, err := os.MkdirTemp(rootdir, ".tmvendor")
	if err != nil {
		return "", errors.E(err, "creating tmp dir inside project")
	}
	defer func() {
		if err := os.RemoveAll(tmTempDir); err != nil {
			log.Warn().Err(err).
				Msg("deleting temp dir inside terramate project")
		}
	}()

	// Same strategy used on the Go toolchain:
	// - https://github.com/golang/go/blob/2ebe77a2fda1ee9ff6fd9a3e08933ad1ebaea039/src/cmd/go/internal/get/get.go#L129

	env := append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
	g, err := git.WithConfig(git.Config{
		WorkingDir:     clonedRepoDir,
		AllowPorcelain: true,
		Env:            env,
	})
	if err != nil {
		return "", err
	}

	event := event.VendorProgress{
		Message:   "downloading",
		TargetDir: modvendor.TargetDir(vendorDir, modsrc),
		Module:    modsrc,
	}
	if !events.Send(event) {
		log.Debug().
			Str("message", event.Message).
			Stringer("targetDir", event.TargetDir).
			Str("module", event.Module.Raw).
			Msg("dropped progress event, event handler is not fast enough or absent")
	}

	if err := g.Clone(modsrc.URL, clonedRepoDir); err != nil {
		return "", err
	}

	const create = false

	if err := g.Checkout(modsrc.Ref, create); err != nil {
		return "", errors.E(err, "checking ref %s", modsrc.Ref)
	}

	if err := os.RemoveAll(filepath.Join(clonedRepoDir, ".git")); err != nil {
		return "", errors.E(err, "removing .git dir from cloned repo")
	}

	matcher, err := manifest.LoadFileMatcher(clonedRepoDir)
	if err != nil {
		return "", err
	}

	const pathSeparator string = string(os.PathSeparator)

	fileFilter := func(path string, entry os.DirEntry) bool {
		if entry.IsDir() {
			return true
		}
		abspath := filepath.Join(path, entry.Name())
		relpath := strings.TrimPrefix(abspath, clonedRepoDir+pathSeparator)
		return matcher.Match(strings.Split(relpath, pathSeparator), entry.IsDir())
	}

	if err := fs.CopyDir(tmTempDir, clonedRepoDir, fileFilter); err != nil {
		return "", errors.E(err, "copying cloned module")
	}

	if err := os.MkdirAll(filepath.Dir(modVendorDir), 0775); err != nil {
		return "", errors.E(err, "creating mod dir inside vendor")
	}

	if err := os.Rename(tmTempDir, modVendorDir); err != nil {
		// Assuming that the whole Terramate project is inside the
		// same fs/mount/dev.
		return "", errors.E(err, "moving module from tmp dir to vendor")
	}
	return modVendorDir, nil
}

func patchFiles(rootdir string, files []string, sources *sourcesInfo) error {
	errs := errors.L()
	for _, fname := range files {
		bytes, err := os.ReadFile(fname)
		if err != nil {
			errs.Append(err)
			continue
		}
		parsedFile, diags := hclwrite.ParseConfig(bytes, fname, hhcl.Pos{})
		if diags.HasErrors() {
			errs.Append(errors.E(diags))
			continue
		}

		blocks := parsedFile.Body().Blocks()
		for _, block := range blocks {
			if block.Type() != "module" {
				continue
			}
			if len(block.Labels()) != 1 {
				continue
			}
			source := block.Body().GetAttribute("source")
			if source == nil {
				continue
			}

			sourceString := string(source.Expr().BuildTokens(nil).Bytes())
			sourceString = strings.TrimSpace(sourceString)
			sourceString = sourceString[1 : len(sourceString)-1] // unquote
			// TODO(i4k): improve to support parenthesis.

			if info, ok := sources.set[sourceString]; ok && info.vendoredAt.String() != "" {
				relPath, err := filepath.Rel(
					filepath.Dir(fname), filepath.Join(rootdir, filepath.FromSlash(info.vendoredAt.String())),
				)
				if err != nil {
					errs.Append(err)
					continue
				}

				relPath = filepath.Join(relPath, info.subdir)
				block.Body().SetAttributeValue("source", cty.StringVal(filepath.ToSlash(relPath)))
			}
		}

		st, err := os.Stat(fname)
		errs.Append(err)
		if err == nil {
			errs.Append(os.WriteFile(fname, parsedFile.Bytes(), st.Mode()))
		}
	}
	return errs.AsError()
}
