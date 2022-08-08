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
	"os"
	"path/filepath"

	"github.com/mineiros-io/terramate/errors"
	"github.com/mineiros-io/terramate/fs"
	"github.com/mineiros-io/terramate/git"
	"github.com/mineiros-io/terramate/tf"
	"github.com/rs/zerolog/log"
)

const (
	// ErrAlreadyVendored indicates that a module is already vendored.
	ErrAlreadyVendored errors.Kind = "module is already vendored"
)

// Vendor will vendor the given module inside the provided root dir.
// The root dir must be an absolute path.
//
// Vendored modules will be located at:
//
// - <rootdir>/vendor/<Source.Path>/<Source.Ref>
//
// If the project is already vendored an error of kind ErrAlreadyVendored will
// be returned, vendored projects are never updated.
//
// The whole path inside the vendor dir will be created if it not exists.
// Vendoring is not recursive, so dependencies won't have their dependencies vendored.
// Vendoring will also not download any git submodules.
//
// It returns the absolute path where the module has been vendored.
func Vendor(rootdir string, modsrc tf.Source) (string, error) {
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

	modVendorDir := filepath.Join(rootdir, "vendor", modsrc.Path, modsrc.Ref)
	if _, err := os.Stat(modVendorDir); err == nil {
		return "", errors.E(ErrAlreadyVendored, "dir %q exists", modVendorDir)
	}

	logger.Trace().Msg("setting up tmp workdir where module will be cloned")

	// We want an initial temporary dir outside of the Terramate project
	// to do the clone since some git setups will assume that any
	// git clone inside a repo is a submodule.
	clonedRepoDir, err := os.MkdirTemp("", "terramate-vendor")
	if err != nil {
		return "", errors.E(err, "creating default tmp dir on host")
	}
	defer func() {
		if err := os.RemoveAll(clonedRepoDir); err != nil {
			log.Warn().Err(err).
				Msg("deleting tmp cloned repo dir")
		}
	}()

	// We want a temporary dir inside the project to where we are going to copy
	// the cloned module first. The idea is that if the copying fails we won't
	// leave any changes on the project vendor dir. The final step then will
	// be atomic using rename, which probably wont fail since the tmpdir is
	// inside the project and the whole project is most likely on the same fs/device.
	tmTempDir, err := os.MkdirTemp(rootdir, ".tmvendor")
	if err != nil {
		return "", errors.E(err, "creating tmp dir inside project")
	}
	defer func() {
		if err := os.RemoveAll(tmTempDir); err != nil {
			log.Warn().Err(err).
				Msg("deleting base tmp dir inside project")
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

	logger.Trace().Msg("moving cloned mod terramate tmp dir")
	if err := fs.CopyDir(tmTempDir, clonedRepoDir,
		func(os.DirEntry) bool { return true }); err != nil {
		return "", errors.E(err, "copying cloned module")
	}

	if err := os.MkdirAll(filepath.Dir(modVendorDir), 0775); err != nil {
		return "", errors.E(err, "creating mod dir inside vendor")
	}

	logger.Trace().Msg("moving cloned mod from workdir to clonedir")
	if err := os.Rename(tmTempDir, modVendorDir); err != nil {
		// Assuming that the whole Terramate project is inside the
		// same fs/mount/dev.
		return "", errors.E(err, "moving module from tmp dir to vendor")
	}
	return modVendorDir, nil
}
