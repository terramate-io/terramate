// Copyright 2022 Mineiros GmbH
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package download is responsible for downloading vendored modules.
package download

import (
	"fmt"
	iofs "io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"github.com/hashicorp/hcl/v2/hclwrite"
	"github.com/mineiros-io/terramate/errors"
	"github.com/mineiros-io/terramate/event"
	"github.com/mineiros-io/terramate/fs"
	"github.com/mineiros-io/terramate/git"
	"github.com/mineiros-io/terramate/modvendor"
	"github.com/mineiros-io/terramate/modvendor/manifest"
	"github.com/mineiros-io/terramate/project"
	"github.com/mineiros-io/terramate/tf"
	"github.com/rs/zerolog/log"
	"github.com/zclconf/go-cty/cty"

	hhcl "github.com/hashicorp/hcl/v2"
)

const (
	// ErrAlreadyVendored indicates that a module is already vendored.
	ErrAlreadyVendored errors.Kind = "module is already vendored"
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
	events EventStream,
) Report {
	report := NewReport(vendorDir)
	if !path.IsAbs(vendorDir.String()) {
		report.Error = errors.E("vendor dir %q must be absolute path", vendorDir)
		return report
	}
	return vendor(rootdir, vendorDir, modsrc, report, nil, events)
}

// VendorAll will vendor all dependencies of the tfdir into rootdir.
// It will scan all .tf files in the directory and vendor each module declaration
// containing the supported remote source URLs.
func VendorAll(
	rootdir string,
	vendorDir project.Path,
	tfdir string,
	events EventStream,
) Report {
	return vendorAll(rootdir, vendorDir, tfdir, NewReport(vendorDir), events)
}

func vendor(
	rootdir string,
	vendorDir project.Path,
	modsrc tf.Source,
	report Report,
	info *modinfo,
	events EventStream,
) Report {
	logger := log.With().
		Str("action", "download.vendor()").
		Str("module.source", modsrc.Raw).
		Logger()

	moddir, err := downloadVendor(rootdir, vendorDir, modsrc, events)
	if err != nil {
		if errors.IsKind(err, ErrAlreadyVendored) {
			// it's not an error in the case it's an indirect vendoring
			if info == nil {
				report.addIgnored(modsrc.Raw, string(ErrAlreadyVendored))
			}
			return report
		}

		reason := errors.E(err, "failed to vendor %q with ref %q",
			modsrc.URL, modsrc.Ref).Error()

		if info != nil {
			reason += fmt.Sprintf(" found in %s", info.origin)
		}

		report.addIgnored(modsrc.Raw, reason)
		return report
	}

	logger.Trace().Msg("successfully downloaded")

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
	events EventStream,
) Report {
	logger := log.With().
		Str("action", "download.vendorAll()").
		Str("dir", tfdir).
		Logger()

	logger.Trace().Msg("scanning .tf files for additional dependencies")

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

		logger.Trace().Str("path", path).Msg("found Terraform file")

		modules, err := tf.ParseModules(path)
		if err != nil {
			errs.Append(err)
			return nil
		}

		for _, mod := range modules {
			logger = logger.With().
				Str("module.source", mod.Source).
				Logger()

			if mod.IsLocal() || mod.Source == "" {
				logger.Trace().Msg("ignoring local module")
				continue
			}

			logger.Trace().Msg("found remote module")

			originMap[path] = struct{}{}

			sources.append(mod.Source, path)
		}
		return nil
	})

	errs.Append(err)

	for _, info := range sources.list {
		source := info.source
		logger := logger.With().
			Str("module.source", source).
			Str("origin", info.origin).
			Logger()

		modsrc, err := tf.ParseSource(source)
		if err != nil {
			report.addIgnored(source, err.Error())
			sources.delete(source)
			continue
		}

		info.subdir = modsrc.Subdir

		targetVendorDir := modvendor.AbsVendorDir(rootdir, vendorDir, modsrc)
		st, err := os.Stat(targetVendorDir)
		if err == nil && st.IsDir() {
			logger.Trace().Msg("already vendored")
			info.vendoredAt = modvendor.TargetDir(vendorDir, modsrc)
			continue
		}

		report = vendor(rootdir, vendorDir, modsrc, report, info, events)
		if v, ok := report.Vendored[modvendor.TargetDir(vendorDir, modsrc)]; ok {
			info.vendoredAt = modvendor.TargetDir(vendorDir, modsrc)
			info.subdir = v.Source.Subdir

			logger.Trace().Msg("vendored successfully")
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
	events EventStream,
) (string, error) {
	logger := log.With().
		Str("action", "download.downloadVendor()").
		Str("rootdir", rootdir).
		Stringer("vendordir", vendorDir).
		Str("url", modsrc.URL).
		Str("path", modsrc.Path).
		Str("ref", modsrc.Ref).
		Logger()

	if modsrc.Ref == "" {
		// TODO(katcipis): handle default references.
		// for now always explicit is fine.
		return "", errors.E("src %v reference must be non-empty", modsrc)
	}

	modVendorDir := modvendor.AbsVendorDir(rootdir, vendorDir, modsrc)
	if _, err := os.Stat(modVendorDir); err == nil {
		return "", errors.E(ErrAlreadyVendored, "dir %q exists", modVendorDir)
	}

	logger.Trace().Msg("setting up temp dir where module will be cloned")

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
	// leave any changes on the project vendor dir. The final step then will
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

	logger = logger.With().
		Str("clonedRepoDir", clonedRepoDir).
		Str("modVendorDir", modVendorDir).
		Str("tmTempDir", tmTempDir).
		Logger()

	logger.Trace().Msg("setting up git wrapper")

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

	event := event.VendorProgressEvent{
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

	logger.Trace().Msg("cloning to workdir")

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

	logger.Trace().Msg("checking for manifest")

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

	logger.Trace().Msg("copying cloned mod to terramate temp vendor dir")
	if err := fs.CopyDir(tmTempDir, clonedRepoDir, fileFilter); err != nil {
		return "", errors.E(err, "copying cloned module")
	}

	if err := os.MkdirAll(filepath.Dir(modVendorDir), 0775); err != nil {
		return "", errors.E(err, "creating mod dir inside vendor")
	}

	logger.Trace().Msg("moving cloned mod from terramate temp vendor to final vendor")
	if err := os.Rename(tmTempDir, modVendorDir); err != nil {
		// Assuming that the whole Terramate project is inside the
		// same fs/mount/dev.
		return "", errors.E(err, "moving module from tmp dir to vendor")
	}
	return modVendorDir, nil
}

func patchFiles(rootdir string, files []string, sources *sourcesInfo) error {
	logger := log.With().
		Str("action", "download.patchFiles").
		Logger()

	logger.Trace().Msg("patching vendored files")

	errs := errors.L()
	for _, fname := range files {
		logger := logger.With().
			Str("filename", fname).
			Logger()

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

		logger.Trace().Msg("successfully parsed for patching")

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

			if info, ok := sources.set[sourceString]; ok && info.vendoredAt != "" {
				logger.Trace().
					Str("module.source", sourceString).
					Msg("found relevant module")

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
