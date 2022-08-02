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
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/mineiros-io/terramate/errors"
	"github.com/mineiros-io/terramate/git"
	"github.com/rs/zerolog/log"
)

// Source represents a module source
type Source struct {
	// URL is the Git URL of the source.
	URL string

	// Path is the path of the source URL. It includes the domain of the URL on it.
	// Eg. github.com/mineiros-io/example
	Path string

	// Ref is the specific reference of this source, if any.
	Ref string
}

const (
	// ErrUnsupportedModSrc indicates that a module source string is invalid.
	ErrUnsupportedModSrc errors.Kind = "unsupported module source"

	// ErrInvalidModSrc indicates that a module source string is invalid.
	ErrInvalidModSrc errors.Kind = "invalid module source"
)

// Vendor will vendor the given module inside the provided vendor
// dir. The vendor dir must be an absolute path.
/// If the project is already vendored it will do nothing and return as a success.
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
func Vendor(vendordir string, src Source) (string, error) {
	logger := log.With().
		Str("action", "modvendor.Vendor()").
		Str("vendordir", vendordir).
		Str("URL", src.URL).
		Str("path", src.Path).
		Str("ref", src.Ref).
		Logger()

	if src.Ref == "" {
		// TODO(katcipis): handle default references.
		// for now always explicit is fine.
		return "", errors.E("src %v reference must be non-empty", src)
	}

	// TODO(katcipis): test that if vendor contains path with matching ref
	// it will do nothing.

	logger.Trace().Msg("setting up tmp workdir")

	workdir, err := os.MkdirTemp("", "terramate-vendor")
	if err != nil {
		return "", errors.E(err, "creating workdir")
	}
	// We ignore the error here since after the final os.Rename
	// the workdir will be moved and won't exist.
	defer os.Remove(workdir)

	clonedir := filepath.Join(vendordir, src.Path, src.Ref)

	logger = logger.With().
		Str("workdir", workdir).
		Str("clonedir", clonedir).
		Logger()

	logger.Trace().Msg("setting up git wrapper")

	g, err := git.WithConfig(git.Config{
		WorkingDir: workdir,
	})
	if err != nil {
		return "", err
	}

	logger.Trace().Msg("cloning to workdir")

	if err := g.Clone(src.URL, workdir); err != nil {
		return "", err
	}

	// This may leave intermediary created dirs hanging on vendordir
	// since we just create all and then delete clone dir.
	// If we get a lot of errors from os.Rename we may need to handle this
	// more gracefully, assuming that os.Rename errors are rare since both
	// dirs where just created.

	if err := os.MkdirAll(filepath.Dir(clonedir), 0775); err != nil {
		return "", errors.E(err, "creating mod dir inside vendor")
	}

	logger.Trace().Msg("moving cloned mod from workdir to clonedir")
	if err := os.Rename(workdir, clonedir); err != nil {
		errs := errors.L()
		errs.Append(errors.E(err, "moving cloned module"))
		errs.Append(os.Remove(clonedir))
		return "", errs.AsError()
	}
	return clonedir, nil
}

// ParseSource parses the given modsource string.
// The modsource must be a valid Terraform Git/Github source reference as documented in:
//
// - https://www.terraform.io/language/modules/sources
//
// Source references that are not Git/Github are not supported.
func ParseSource(modsource string) (Source, error) {
	ref := ""
	splitParams := strings.Split(modsource, "?")
	modsource = splitParams[0]

	if len(splitParams) > 1 {
		if len(splitParams) != 2 {
			return Source{}, errors.E(ErrInvalidModSrc, "unexpected extra '?' on source")
		}

		refParam := splitParams[1]
		splitRefParam := strings.Split(refParam, "=")

		if len(splitRefParam) != 2 {
			return Source{}, errors.E(ErrInvalidModSrc, "parsing ref param %q", refParam)
		}

		if splitRefParam[0] != "ref" {
			return Source{}, errors.E(ErrInvalidModSrc, "unknown param %q", splitRefParam[0])
		}

		ref = splitRefParam[1]
		if ref == "" {
			return Source{}, errors.E(ErrInvalidModSrc, "ref param %q is empty", refParam)
		}
	}

	switch {
	case strings.HasPrefix(modsource, "github.com"):
		return Source{
			URL:  fmt.Sprintf("https://%s.git", modsource),
			Path: modsource,
			Ref:  ref,
		}, nil
	case strings.HasPrefix(modsource, "git@"):
		// In git it could be any user@host, but here we are supporting
		// the specific options allowed by Terraform on module.source:
		// - https://www.terraform.io/language/modules/sources#github
		// In this case being Github ssh access.
		return Source{
			URL:  modsource,
			Path: parseGithubAtPath(modsource),
			Ref:  ref,
		}, nil
	case strings.HasPrefix(modsource, "git::"):
		// Generic git: https://www.terraform.io/language/modules/sources#generic-git-repository
		path, err := parseGitColonPath(modsource)
		if err != nil {
			return Source{}, err
		}
		return Source{
			URL:  strings.TrimPrefix(modsource, "git::"),
			Path: path,
			Ref:  ref,
		}, nil

	default:
		return Source{}, errors.E(ErrUnsupportedModSrc)
	}
}

func parseGithubAtPath(modsource string) string {
	path := strings.TrimPrefix(modsource, "git@")
	path = strings.TrimSuffix(path, ".git")
	// This is GH specific, so we don't need to handle specific ports
	// and we can assume it is always git@github.com:org/path.git.
	return strings.Replace(path, ":", "/", 1)
}

func parseGitColonPath(modsource string) (string, error) {
	rawURL := strings.TrimPrefix(modsource, "git::")
	u, err := url.Parse(rawURL)
	if err != nil {
		return "", errors.E(ErrInvalidModSrc, "invalid URL inside git::")
	}
	path := filepath.Join(u.Host, u.Path)
	return strings.TrimSuffix(path, ".git"), nil
}
