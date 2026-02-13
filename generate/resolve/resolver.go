// Copyright 2025 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package resolve

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/hashicorp/go-getter"
	"github.com/rs/zerolog/log"
	"github.com/terramate-io/terramate/di"
	"github.com/terramate-io/terramate/errors"
	"github.com/terramate-io/terramate/fs"
	"github.com/terramate-io/terramate/project"
)

// Directory constants for package installation paths.
const (
	ComponentsDir = "components"
	BundlesDir    = "bundles"
	SchemasDir    = "schemas"
	ManifestsDir  = "manifests"
)

// Resolver implements the API interface for resolving package sources.
type Resolver struct {
	cachedir string
}

// NewAPI creates a factory for the resolve API.
func NewAPI(cachedir string) di.Factory[API] {
	return func(_ context.Context) (API, error) {
		return &Resolver{
			cachedir: cachedir,
		}, nil
	}
}

// Resolve returns the project-relative location for the given source.
// If src starts with a `/`, it is a local path that is looked up relative to the repository root.
// Otherwise, it is considered a remote source. See
// https://developer.hashicorp.com/terraform/language/modules/sources
func (r *Resolver) Resolve(rootdir string, src string, kind Kind, allowFetch bool, opts ...Option) (project.Path, error) {
	optVals := OptionValues{}
	for _, opt := range opts {
		opt(r, &optVals)
	}

	logger := log.With().
		Str("action", "source.Resolve()").
		Str("src", src).
		Str("parent_src", optVals.ParentSrc).
		Int("kind", int(kind)).
		Bool("allow_fetch", allowFetch).
		Logger()

	src = CombineSources(src, optVals.ParentSrc)

	logger.Debug().Msgf("src with parent path adjustment: %s", src)

	if strings.HasPrefix(src, "/") {
		p := project.NewPath(src)
		logger.Debug().Msgf("resolved to project path %s", p.String())
		return p, nil
	}

	// TODO(snk): We assume parentSrc is either abs or url here. rel should be handled, too (probably with an error).

	return r.resolveRemote(rootdir, src, kind, allowFetch)
}

func (r *Resolver) resolveRemote(rootdir, src string, kind Kind, allowFetch bool) (project.Path, error) {
	logger := log.With().
		Str("action", "source.resolveRemote()").
		Str("src", src).
		Int("kind", int(kind)).
		Bool("allow_fetch", allowFetch).
		Logger()

	detectors := []getter.Detector{
		new(getter.GitHubDetector),
		new(getter.GitLabDetector),
		new(getter.GitDetector),
		new(getter.BitBucketDetector),
		new(getter.S3Detector),
		new(getter.GCSDetector),
	}

	var err error
	src, err = getter.Detect(src, rootdir, detectors)
	if err != nil {
		return project.Path{}, errors.E(err, "detecting source kind")
	}

	srcDir, srcSubdir := getter.SourceDirSubdir(src)
	pkgCacheDir := r.getPackageCacheDir(srcDir)

	// TODO: Update, even if exists.
	if _, err := os.Stat(pkgCacheDir); err != nil {
		logger.Debug().Msgf("package does not exist in cache dir %s, fetching...", pkgCacheDir)

		if !allowFetch {
			return project.Path{}, errors.E("package for source could not be found: %s", src)
		}
		err = getter.GetAny(pkgCacheDir, srcDir, getter.WithDetectors(detectors))
		if err != nil {
			return project.Path{}, errors.E(err, "getting package")
		}
	}

	pkgInstallDir := r.getPackageInstallDir(srcDir, srcSubdir, "TODO", kind)
	absPkgInstallDir := project.AbsPath(rootdir, pkgInstallDir.String())

	// Already exists.
	if _, err := os.Stat(absPkgInstallDir); err == nil {
		return pathForSourceKind(pkgInstallDir, kind), nil
	}

	logger.Debug().Msgf("package does not exist in install dir %s, installing...", absPkgInstallDir)

	// Copy to tmpdir, then rename so its an atomic operation.
	tmTempDir, err := os.MkdirTemp(rootdir, ".tm_pkg")
	if err != nil {
		return project.Path{}, errors.E(err, "creating tmp dir inside project")
	}
	defer func() {
		if err := os.RemoveAll(tmTempDir); err != nil {
			log.Warn().Err(err).
				Msg("deleting temp dir inside terramate project")
		}
	}()

	pkgCacheSubdir := filepath.Join(pkgCacheDir, srcSubdir)

	if _, err := os.Stat(pkgCacheSubdir); err == os.ErrNotExist {
		return project.Path{}, errors.E(err, "source directory does not exist in repository")
	}

	if err := fs.CopyDir(tmTempDir, pkgCacheSubdir, filterForSourceKind(kind)); err != nil {
		return project.Path{}, errors.E(err, "copying package")
	}

	if err := os.MkdirAll(filepath.Dir(absPkgInstallDir), 0775); err != nil {
		return project.Path{}, errors.E(err, "creating package dir")
	}

	if err := os.Rename(tmTempDir, absPkgInstallDir); err != nil {
		return project.Path{}, errors.E(err, "moving package from tmp dir to install location")
	}

	return pathForSourceKind(pkgInstallDir, kind), nil
}

func filterForSourceKind(kind Kind) func(path string, entry os.DirEntry) bool {
	switch kind {
	case Bundle, Component, Schema:
		return func(_ string, entry os.DirEntry) bool {
			return !strings.HasPrefix(entry.Name(), ".")
		}
	case Manifest:
		return func(_ string, entry os.DirEntry) bool {
			if entry.IsDir() {
				return true
			}
			return entry.Name() == "terramate_packages.json"
		}
	default:
		panic("unknown source kind")
	}
}

func pathForSourceKind(pkgInstallDir project.Path, kind Kind) project.Path {
	if kind == Manifest {
		return pkgInstallDir.Join("terramate_packages.json")
	}
	return pkgInstallDir
}

// getPackageCacheDir returns the cache folder for the given source.
// This is a location outside of the Terramate project and shared by multiple projects.
// The cache folder is the same for different subdirs of the same base source,
// i.e. a Git repo is only cloned once if it contains multiple bundles.
func (r *Resolver) getPackageCacheDir(srcDir string) string {
	hasher := md5.New()
	hasher.Write([]byte(srcDir))
	repokey := hex.EncodeToString(hasher.Sum(nil))

	return path.Join(r.cachedir, repokey)
}

// getPackageCacheDir returns the install location for the given source.
// This is a unique location inside the Terramate project.
// Each bundle/component/manifest has its own folder.
func (r *Resolver) getPackageInstallDir(srcDir, srcSubdir string, digest string, kind Kind) project.Path {
	var kindDir string
	switch kind {
	case Bundle:
		kindDir = BundlesDir
	case Component:
		kindDir = ComponentsDir
	case Schema:
		kindDir = SchemasDir
	case Manifest:
		kindDir = ManifestsDir
	default:
		panic("unknown source kind")
	}

	hasher := md5.New()
	hasher.Write([]byte(srcDir))
	hasher.Write([]byte(srcSubdir))
	hasher.Write([]byte(digest))
	pkgkey := hex.EncodeToString(hasher.Sum(nil))

	return project.NewPath(path.Join("/.terramate", kindDir, string(pkgkey)))
}

// CombineSources combines two sources based on specific rules.
// See comments in implementation for details.
func CombineSources(src, parentSrc string) string {
	if parentSrc == "" {
		return src
	}

	if strings.HasPrefix(src, ".") {
		if strings.HasPrefix(parentSrc, ".") {
			// src=rel, parent=rel
			// Join them, return rel.
			r := path.Clean(path.Join(parentSrc, src))
			if !strings.HasPrefix(r, ".") {
				return "./" + r
			}
			return r
		} else if strings.HasPrefix(parentSrc, "/") {
			// src=rel, parent=abs
			// Join them, return abs.
			return path.Clean(path.Join(parentSrc, src))
		}
		// src=rel, parent=url
		// Join src to subdir part of parent, return url.
		parentDir, parentSubdir := getter.SourceDirSubdir(parentSrc)
		// Add / to prevent leading ..'s
		r := path.Clean(path.Join("/", parentSubdir, src))
		r, _ = strings.CutPrefix(r, "/")
		return makeDirSubdir(parentDir, r)
	} else if strings.HasPrefix(src, "/") {
		if strings.HasPrefix(parentSrc, ".") || strings.HasPrefix(parentSrc, "/") {
			// src=abs, parent=rel or abs
			// Use src, discard parent.
			return src
		}
		// src=abs, parent=url
		// Replace subdir part of parent with src, return url.
		parentDir, _ := getter.SourceDirSubdir(parentSrc)
		return makeDirSubdir(parentDir, src[1:])
	}
	// src=url
	// Use src, discard parent.
	return src
}

func makeDirSubdir(dir, subdir string) string {
	if subdir == "" || subdir == "." {
		return dir
	}
	path, params, found := strings.Cut(dir, "?")
	if found {
		return fmt.Sprintf("%s//%s?%s", path, subdir, params)
	}
	return fmt.Sprintf("%s//%s", path, subdir)
}
