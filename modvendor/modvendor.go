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
	"github.com/mineiros-io/terramate/git"
	"github.com/mineiros-io/terramate/tf"
	"github.com/rs/zerolog/log"
)

const (
	// ErrAlreadyVendored indicates that a module is already vendored.
	ErrAlreadyVendored errors.Kind = "module is already vendored"
)

// Vendor will vendor the given module inside the provided vendor
// dir. The vendor dir must be an absolute path.
//
// If the project is already vendored an error of kind ErrAlreadyVendored will
// be returned, vendored projects are never updated.
//
// Vendored modules will be located at:
//
// - <vendordir>/<Source.Path>/<Source.Ref>
//
// If the provided source has no reference the provided Source.URL will be
// used to retrieve the default remote branch to be used as reference.
//
// The whole path inside the vendor dir will be created if it not exists.
// Vendoring is not recursive, so dependencies won't have their dependencies vendored.
// Vendoring will also not download any git submodules.
//
// It returns the absolute path where the code has been vendored, which will be inside
// the given vendordir.
func Vendor(vendordir string, modsrc tf.Source) (string, error) {
	logger := log.With().
		Str("action", "modvendor.Vendor()").
		Str("vendordir", vendordir).
		Str("url", modsrc.URL).
		Str("path", modsrc.Path).
		Str("ref", modsrc.Ref).
		Logger()

	if modsrc.Ref == "" {
		// TODO(katcipis): handle default references.
		// for now always explicit is fine.
		return "", errors.E("src %v reference must be non-empty", modsrc)
	}

	clonedir := filepath.Join(vendordir, modsrc.Path, modsrc.Ref)
	if _, err := os.Stat(clonedir); err == nil {
		return "", errors.E(ErrAlreadyVendored, "dir %q exists", clonedir)
	}

	logger.Trace().Msg("setting up tmp workdir")

	workdir, err := os.MkdirTemp("", "terramate-vendor")
	if err != nil {
		return "", errors.E(err, "creating workdir")
	}
	defer func() {
		// We ignore the error here since after the final os.Rename
		// the workdir will be moved and won't exist.
		_ = os.Remove(workdir)
	}()

	logger = logger.With().
		Str("workdir", workdir).
		Str("clonedir", clonedir).
		Logger()

	logger.Trace().Msg("setting up git wrapper")

	g, err := git.WithConfig(git.Config{
		WorkingDir:     workdir,
		AllowPorcelain: true,
	})
	if err != nil {
		return "", err
	}

	logger.Trace().Msg("cloning to workdir")

	if err := g.Clone(modsrc.URL, workdir); err != nil {
		return "", err
	}

	// TODO(katcipis): now we need to checkout the correct branch/commitID

	if err := os.RemoveAll(filepath.Join(workdir, ".git")); err != nil {
		return "", errors.E(err, "removing .git dir from cloned repo")
	}

	if err := os.MkdirAll(filepath.Dir(clonedir), 0775); err != nil {
		return "", errors.E(err, "creating mod dir inside vendor")
	}

	logger.Trace().Msg("moving cloned mod from workdir to clonedir")
	if err := os.Rename(workdir, clonedir); err != nil {
		// This may leave intermediary created dirs hanging on vendordir
		// since we just create all and then delete clone dir on a failure to move.
		// If we get a lot of errors from os.Rename we may need to handle this
		// more gracefully, here we assume that os.Rename errors are rare since both
		// dirs were just created.
		errs := errors.L()
		errs.Append(errors.E(err, "moving cloned module"))
		errs.Append(os.Remove(clonedir))
		return "", errs.AsError()
	}
	return clonedir, nil
}
