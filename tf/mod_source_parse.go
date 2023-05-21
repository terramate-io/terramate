// Copyright 2023 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package tf

import (
	"net/url"
	"path"
	"strings"

	"github.com/terramate-io/terramate/errors"
)

// Source represents a module source
type Source struct {
	// URL is the Git URL of the source.
	URL string

	// Path is the path of the source URL. It includes the domain of the URL on it.
	// Eg. github.com/terramate-io/example
	Path string

	// PathScheme is the scheme of the path part.
	PathScheme string

	// Subdir is the subdir component of the source path, if any, as defined
	// here: https://www.terraform.io/language/modules/sources#modules-in-package-sub-directories
	Subdir string

	// Ref is the specific reference of this source, if any.
	Ref string

	// Raw source
	Raw string
}

const (
	// ErrUnsupportedModSrc indicates that a module source string is unsupported.
	ErrUnsupportedModSrc errors.Kind = "unsupported module source"

	// ErrInvalidModSrc indicates that a module source string is invalid.
	ErrInvalidModSrc errors.Kind = "invalid module source"
)

// ParseSource parses the given modsource string.
// The modsource must be a valid Terraform Git/Github source reference as documented in:
//
// - https://www.terraform.io/language/modules/sources
//
// Source references that are not Git/Github are not supported.
func ParseSource(modsource string) (Source, error) {
	switch {
	case strings.HasPrefix(modsource, "github.com"):
		u, err := url.Parse(modsource)
		if err != nil {
			return Source{}, errors.E(ErrInvalidModSrc, err,
				"%s is not a URL", modsource)
		}
		ref := u.Query().Get("ref")
		subdir := parseURLSubdir(u)
		u.RawQuery = ""
		u.Scheme = "https"
		u.Path = strings.TrimSuffix(u.Path, ".git")

		path := path.Join(u.Host, u.Path)
		return Source{
			Raw:        modsource,
			URL:        u.String() + ".git",
			Path:       path,
			PathScheme: u.Scheme,
			Subdir:     subdir,
			Ref:        ref,
		}, nil

	case strings.HasPrefix(modsource, "git@"):
		// In a git scp like url it could be any user@host, but here we are supporting
		// the specific options allowed by Terraform on module.source:
		// - https://www.terraform.io/language/modules/sources#github
		// In this case being Github ssh access, which is always git@.

		rawURL := strings.TrimPrefix(modsource, "git@")

		// This is not a valid URL given the nature of scp strings
		// But it is enough for us to parse the query parameters
		// and form a path that makes sense.
		u, err := url.Parse(rawURL)
		if err != nil {
			return Source{}, errors.E(ErrInvalidModSrc, err,
				"invalid URL inside %s", modsource)
		}

		ref := u.Query().Get("ref")
		u.RawQuery = ""
		pathstr, subdir := parseSubdir(u.Opaque)
		u.Opaque = pathstr
		pathstr = strings.TrimSuffix(path.Join(u.Scheme, u.Opaque), ".git")

		return Source{
			Raw:        modsource,
			URL:        "git@" + u.String(),
			Path:       pathstr,
			PathScheme: "git",
			Subdir:     subdir,
			Ref:        ref,
		}, nil

	case strings.HasPrefix(modsource, "git::"):
		// Generic git: https://www.terraform.io/language/modules/sources#generic-git-repository
		rawURL := strings.TrimPrefix(modsource, "git::")
		u, err := url.Parse(rawURL)
		if err != nil {
			return Source{}, errors.E(ErrInvalidModSrc, modsource)
		}

		if u.Path == "" {
			return Source{}, errors.E(
				ErrInvalidModSrc,
				"source %q is missing the path component",
				modsource,
			)
		}

		subdir := parseURLSubdir(u)
		// We don't want : on the pathstr. So we replace the possible :
		// that can exist on the host.
		pathstr := path.Join(strings.Replace(u.Host, ":", "-", -1), u.Path)
		pathstr = strings.TrimSuffix(pathstr, ".git")

		if err != nil {
			return Source{}, err
		}
		ref := u.Query().Get("ref")
		u.RawQuery = ""
		return Source{
			Raw:        modsource,
			URL:        u.String(),
			Path:       pathstr,
			PathScheme: u.Scheme,
			Subdir:     subdir,
			Ref:        ref,
		}, nil

	default:
		return Source{}, errors.E(ErrUnsupportedModSrc)
	}
}

func parseSubdir(s string) (string, string) {
	if !strings.Contains(s, "//") {
		return s, ""
	}

	// From the specs we should have a single // on the path:
	// https://www.terraform.io/language/modules/sources#modules-in-package-sub-directories
	parsed := strings.Split(s, "//")
	if len(parsed[1]) == 0 {
		return parsed[0], ""
	}
	return parsed[0], "/" + parsed[1]
}

func parseURLSubdir(u *url.URL) string {
	path, subdir := parseSubdir(u.Path)
	u.Path = path
	return subdir
}
