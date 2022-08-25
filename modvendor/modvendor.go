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

package modvendor

import (
	"fmt"
	iofs "io/fs"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclwrite"
	"github.com/mineiros-io/terramate/errors"
	"github.com/mineiros-io/terramate/fs"
	"github.com/mineiros-io/terramate/git"
	"github.com/mineiros-io/terramate/project"
	"github.com/mineiros-io/terramate/tf"
	"github.com/rs/zerolog/log"
	"github.com/zclconf/go-cty/cty"
)

const (
	// ErrAlreadyVendored indicates that a module is already vendored.
	ErrAlreadyVendored errors.Kind = "module is already vendored"
)

// Vendor will vendor the given module and its dependencies inside the provided
// root dir.
// The root dir must be an absolute path.
//
// Vendored modules will be located at:
//
// - <rootdir>/vendor/<Source.Path>/<Source.Ref>
//
//
// The whole path inside the vendor dir will be created if it not exists.
// Vendoring will also not download any git submodules.
//
// It returns a report of everything vendored and ignored (with a reason).
func Vendor(rootdir string, modsrc tf.Source) (report Report) {
	report.Vendored = make(map[string]Vendored)

	moddir, err := vendor(rootdir, modsrc)
	if err != nil {
		if errors.IsKind(err, ErrAlreadyVendored) {
			report.addIgnored(modsrc.Raw, string(ErrAlreadyVendored))
			return report
		}
		report.addIgnored(modsrc.Raw, errors.E(err, "vendoring %q with ref %q",
			modsrc.URL, modsrc.Ref).Error())
		return report
	}

	report.addVendored(modsrc.Raw, modsrc)

	depsReport := VendorAll(rootdir, moddir)
	report = mergeReports(report, depsReport)
	return report
}

// VendorAll will vendor all dependencies of the tfdir into rootdir.
// It will scan all .tf files in the directory and vendor each module declaration
// containing the supported remote source URLs.
func VendorAll(rootdir string, tfdir string) (report Report) {
	type modinfo struct {
		source     string
		vendoredAt string
		origin     string
	}

	report.Vendored = make(map[string]Vendored)

	sourcemap := map[string]*modinfo{}
	originMap := map[string]struct{}{}
	errs := errors.L()
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
			if !mod.IsLocal() {
				sourcemap[mod.Source] = &modinfo{
					source: mod.Source,
					origin: path,
				}
				originMap[path] = struct{}{}
			}
		}

		return nil
	})

	errs.Append(err)

	for source, info := range sourcemap {
		modsrc, err := tf.ParseSource(source)
		if err != nil {
			report.addIgnored(source, err.Error())
			delete(sourcemap, source)
			continue
		}
		subreport := Vendor(rootdir, modsrc)
		for subsource, vendored := range subreport.Vendored {
			report.Vendored[subsource] = vendored
			if vendored.Source.Raw == modsrc.Raw {
				info.vendoredAt = Dir(modsrc)
			}
		}
		for _, ignored := range subreport.Ignored {
			report.addIgnored(source, errors.E(fmt.Errorf(ignored.Reason),
				"failed to vendor %q with ref %q found in %s",
				modsrc.URL, modsrc.Ref, project.PrjAbsPath(rootdir, info.origin)).Error())
			if ignored.RawSource == source {
				delete(sourcemap, source)
			}
		}
	}

	for fname := range originMap {
		bytes, err := ioutil.ReadFile(fname)
		if err != nil {
			errs.Append(err)
			continue
		}
		parsedFile, diags := hclwrite.ParseConfig(bytes, fname, hcl.Pos{})
		if diags.HasErrors() {
			errs.Append(err)
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

			if info, ok := sourcemap[sourceString]; ok {
				relPath, err := filepath.Rel(
					filepath.Dir(fname), filepath.Join(rootdir, info.vendoredAt),
				)
				if err != nil {
					errs.Append(err)
				} else {
					newexpr := hclwrite.NewExpressionLiteral(cty.StringVal(relPath))
					block.Body().SetAttributeRaw("source", newexpr.BuildTokens(nil))
				}
			}
		}

		newcontent := parsedFile.Bytes()
		errs.Append(ioutil.WriteFile(fname, newcontent, 0644))
	}

	report.Error = errs.AsError()
	return report
}

// vendor will vendor the provided modsrc into the rootdir.
// If the project is already vendored an error of kind ErrAlreadyVendored will
// be returned, vendored projects are never updated.
// This function is not recursive, so dependencies won't have their dependencies
// vendored. See Vendor() for a recursive vendoring function.
func vendor(rootdir string, modsrc tf.Source) (string, error) {
	logger := log.With().
		Str("action", "modvendor.Vendor()").
		Str("rootdir", rootdir).
		Str("url", modsrc.URL).
		Str("path", modsrc.Path).
		Str("ref", modsrc.Ref).
		Logger()

	if modsrc.Ref == "" {
		// TODO(katcipis): handle default references.
		// for now always explicit is fine.
		return "", errors.E("src %v reference must be non-empty", modsrc)
	}

	modVendorDir := AbsVendorDir(rootdir, modsrc)
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

	g, err := git.WithConfig(git.Config{
		WorkingDir:     clonedRepoDir,
		AllowPorcelain: true,
		Env:            os.Environ(),
	})
	if err != nil {
		return "", err
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

	logger.Trace().Msg("copying cloned mod to terramate temp vendor dir")
	if err := fs.CopyDir(tmTempDir, clonedRepoDir,
		func(os.DirEntry) bool { return true }); err != nil {
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

func mergeReports(r1, r2 Report) (out Report) {
	out.Vendored = make(map[string]Vendored)
	for k, v := range r1.Vendored {
		out.Vendored[k] = v
	}
	for k, v := range r2.Vendored {
		out.Vendored[k] = v
	}
	out.Ignored = append(out.Ignored, r1.Ignored...)
	out.Ignored = append(out.Ignored, r2.Ignored...)
	errs := errors.L()
	errs.Append(r1.Error, r2.Error)
	out.Error = errs.AsError()
	return out
}

func Dir(modsrc tf.Source) string {
	return filepath.Join("/vendor", modsrc.Path, modsrc.Ref)
}

func AbsVendorDir(rootdir string, modsrc tf.Source) string {
	return filepath.Join(rootdir, Dir(modsrc))
}
