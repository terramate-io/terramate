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
	"path/filepath"
	"strings"

	"github.com/mineiros-io/terramate/errors"
)

// Source represents a module source
type Source struct {
	// Remote is the remote of the source that will be referenced
	// directly when downloading a source.
	Remote string

	// Path is a relative path representation of the source remote
	// independent from any specific reference. It includes the
	// domain of the remote as the root dir.
	// Eg.  github.com/mineiros-io/example
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
// dir. If the project is already vendored it will do nothing and return
// as a success.
//
// Vendored modules will be located at:
//
// - <vendordir>/<domain>/<dir>/<subdir>/<ref>
//
// The whole path inside the vendor dir will be created if it not exists.
// Vendoring is not recursive, so dependencies won't have their dependencies vendored.
// Vendoring will also not download git submodules, if any.
//
// The module source must be a valid Terraform Git/Github source reference as documented in:
//
// - https://www.terraform.io/language/modules/sources
//
// Source references that are not Git/Github are not supported.
func Vendor(vendordir, src Source) error {
	return nil
}

// ParseSource parses the given modsource string. It returns an error if the modsource
// string is unsupported.
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
			Remote: fmt.Sprintf("https://%s.git", modsource),
			Path:   modsource,
			Ref:    ref,
		}, nil
	case strings.HasPrefix(modsource, "git@"):
		// In git it could be any user@host, but here we are supporting
		// the specific options allowed by Terraform on module.source:
		// - https://www.terraform.io/language/modules/sources#github
		// In this case being Github ssh access.
		return Source{
			Remote: modsource,
			Path:   parseGithubAtPath(modsource),
			Ref:    ref,
		}, nil
	case strings.HasPrefix(modsource, "git::"):
		// Generic git: https://www.terraform.io/language/modules/sources#generic-git-repository
		path, err := parseGitColonPath(modsource)
		if err != nil {
			return Source{}, err
		}
		return Source{
			Remote: strings.TrimPrefix(modsource, "git::"),
			Path:   path,
			Ref:    ref,
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
