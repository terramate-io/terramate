// Copyright 2021 Mineiros GmbH
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

package generate

import (
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"github.com/mineiros-io/terramate/config"
	"github.com/mineiros-io/terramate/errors"
	"github.com/mineiros-io/terramate/generate/genfile"
	"github.com/mineiros-io/terramate/generate/genhcl"
	"github.com/mineiros-io/terramate/globals"
	"github.com/mineiros-io/terramate/hcl/eval"
	"github.com/mineiros-io/terramate/hcl/info"
	"github.com/mineiros-io/terramate/project"
	"github.com/mineiros-io/terramate/stack"
	"github.com/rs/zerolog/log"
)

const (
	// ErrLoadingGlobals indicates failure loading globals during code generation.
	ErrLoadingGlobals errors.Kind = "loading globals"

	// ErrManualCodeExists indicates code generation would replace code that
	// was not previously generated by Terramate.
	ErrManualCodeExists errors.Kind = "manually defined code found"

	// ErrConflictingConfig indicates that two code generation configurations
	// are conflicting, like both generates a file with the same name
	// and would overwrite each other.
	ErrConflictingConfig errors.Kind = "conflicting config detected"

	// ErrInvalidGenBlockLabel indicates that a generate block
	// has an invalid label as the target to save the generated code.
	ErrInvalidGenBlockLabel errors.Kind = "invalid generate block label"

	// ErrAssertion indicates that code generation configuration
	// has a failed assertion.
	ErrAssertion errors.Kind = "assertion failed"
)

// GenFile represents a generated file loaded from a Terramate configuration.
type GenFile interface {
	// Header is the header of the generated file, if any.
	Header() string
	// Body is the body of the generated file, if any.
	Body() string
	// Label is the label of the origin generate block that generated this file.
	Label() string
	// Range is the range of the origin generate block that generated this file.
	Range() info.Range
	// Condition is true if the origin generate block had a true condition, false otherwise.
	Condition() bool
	// Asserts is the origin generate block assert blocks.
	Asserts() []config.Assert
}

// LoadResult represents all generated files of a specific directory.
type LoadResult struct {
	// Dir is from where the generated files were loaded, or where a failure occurred
	// if Err is not nil.
	Dir project.Path
	// Files is the generated files for this directory.
	Files []GenFile
	// Err will be non-nil if loading generated files for a specific dir failed
	Err error
}

// Load will load all the generated files inside the given tree.
// Each directory will be represented by a single [LoadResult] inside the returned slice.
// Errors generating code for specific dirs will be found inside each [LoadResult].
// If a critical error that fails the loading of all results happens it returns
// a non-nil error. In this case the error is not specific to generating code for a
// specific dir.
func Load(cfg *config.Tree) ([]LoadResult, error) {
	stacks, err := stack.LoadAll(cfg)
	if err != nil {
		return nil, err
	}
	projmeta := stack.NewProjectMetadata(cfg.RootDir(), stacks)
	results := make(map[project.Path]*LoadResult)

	for _, st := range stacks {
		cfg, _ = cfg.Root().Lookup(st.Path())
		res, ok := results[st.Path()]
		if !ok {
			res = &LoadResult{Dir: st.Path()}
			results[st.Path()] = res
		}

		ctx, err := eval.NewContext(st.HostPath())
		if err != nil {
			res.Err = errors.L(res.Err, err).AsError()
			continue
		}

		ctx.SetNamespace("terramate", stack.MetadataToCtyValues(projmeta, st))
		loadres := globals.Load(cfg, st.Path(), ctx)
		if err := loadres.AsError(); err != nil {
			res.Err = errors.L(res.Err, err).AsError()
			continue
		}

		generated, err := loadStackCodeCfgs(cfg, ctx)
		if err != nil {
			res.Err = errors.L(res.Err, err).AsError()
			continue
		}
		res.Files = append(res.Files, generated...)
	}
	var resultList []LoadResult
	for _, r := range results {
		resultList = append(resultList, *r)
	}
	sort.Slice(resultList, func(i, j int) bool {
		return resultList[i].Dir.String() < resultList[j].Dir.String()
	})
	return resultList, nil
}

// Do will walk all the stacks inside the given working dir
// generating code for any stack it finds as it goes along.
//
// Code is generated based on configuration files spread around the entire
// project until it reaches the given root. So even though a configuration
// file may be outside the given working dir it may be used on code generation
// if it is in a dir that is a parent of a stack found inside the working dir.
//
// The provided root must be the project's root directory as an absolute path.
// The provided working dir must be an absolute path that is a child of the
// provided root (or the same as root, indicating that working dir is the project root).
//
// It will return a report including details of which stacks succeed and failed
// on code generation, any failure found is added to the report but does not abort
// the overall code generation process, so partial results can be obtained and the
// report needs to be inspected to check.
func Do(root *config.Tree, workingDir string) Report {
	report := forEachDir(root, workingDir, func(
		dir project.Path,
		evalctx *eval.Context,
	) dirReport {
		dirHostPath := project.AbsPath(root.RootDir(), dir.String())
		logger := log.With().
			Str("action", "generate.Do()").
			Stringer("dir", dir).
			Logger()

		report := dirReport{}

		logger.Debug().Msg("generating files")

		cfg, ok := root.Lookup(dir)
		if !ok || !cfg.IsStack() {
			panic("unreachable")
		}
		asserts, err := loadAsserts(cfg, evalctx)
		if err != nil {
			report.err = err
			return report
		}

		var generated []GenFile
		if cfg.IsStack() {
			genfiles, err := loadStackCodeCfgs(cfg, evalctx)
			if err != nil {
				report.err = err
				return report
			}
			generated = append(generated, genfiles...)
		} else {
			panic("bug")
		}

		for _, gen := range generated {
			asserts = append(asserts, gen.Asserts()...)
		}

		errs := errors.L()
		for _, assert := range asserts {
			if !assert.Assertion {
				assertRange := assert.Range
				assertRange.Filename = project.PrjAbsPath(root.RootDir(), assert.Range.Filename).String()
				if assert.Warning {
					log.Warn().
						Stringer("origin", assertRange).
						Str("msg", assert.Message).
						Stringer("stack", dir).
						Msg("assertion failed")
				} else {
					msg := fmt.Sprintf("%s: %s", assertRange, assert.Message)

					logger.Debug().Msgf("assertion failure detected: %s", msg)

					err := errors.E(ErrAssertion, msg)
					errs.Append(err)
				}
			}
		}

		if err := errs.AsError(); err != nil {
			report.err = err
			return report
		}

		err = validateGeneratedFiles(cfg, generated)
		if err != nil {
			report.err = err
			return report
		}

		var removedFiles map[string]string

		failureReport := func(r dirReport, err error) dirReport {
			r.err = err
			for filename := range removedFiles {
				r.addDeletedFile(filename)
			}
			return r
		}

		removedFiles, err = removeStackGeneratedFiles(cfg, dirHostPath, generated)
		if err != nil {
			return failureReport(
				report,
				errors.E(err, "removing old generated files"),
			)
		}

		logger.Debug().Msg("saving generated files")

		for _, file := range generated {
			if !file.Condition() {
				continue
			}

			filename := file.Label()
			path := filepath.Join(dirHostPath, filename)
			logger := logger.With().
				Str("filename", filename).
				Logger()

			if !file.Condition() {
				logger.Debug().Msg("condition is false, ignoring file")
				continue
			}

			err := writeGeneratedCode(path, file)
			if err != nil {
				return failureReport(
					report,
					errors.E(err, "saving file %q", filename),
				)
			}

			// Change detection + remove entries that got re-generated
			removedFileBody, ok := removedFiles[filename]
			if !ok {
				log.Info().
					Stringer("dir", dir).
					Str("file", filename).
					Msg("created file")

				report.addCreatedFile(filename)
			} else {
				body := file.Header() + file.Body()
				if body != removedFileBody {
					log.Info().
						Stringer("dir", dir).
						Str("file", filename).
						Msg("changed file")

					report.addChangedFile(filename)
				}
				delete(removedFiles, filename)
			}
		}

		for filename := range removedFiles {
			log.Info().
				Stringer("dir", dir).
				Str("file", filename).
				Msg("deleted file")
			report.addDeletedFile(filename)
		}

		logger.Debug().Msg("finished generating files")
		return report
	})

	return cleanupOrphaned(root, report)
}

// ListGenFiles will list the path of all generated code inside the given dir
// and all its subdirs that are not stacks. The returned paths are relative to
// the given dir, like:
//
//   - filename.hcl
//   - dir/filename.hcl
//
// The filenames are ordered lexicographically. They always use slash (/) as a
// dir separator (independent on the OS).
//
// dir must be an absolute path and must be inside the given rootdir.
//
// When called with a dir that is not a stack this function will provide a list
// of all orphaned generated files inside this dir, since it won't search inside any stacks.
// So calling with dir == rootdir will provide all orphaned files inside a project.
//
// When called with a dir that is a stack this function will list all generated
// files that are owned by the stack, since it won't search inside any child stacks.
func ListGenFiles(cfg *config.Tree, dir string) ([]string, error) {
	pendingSubDirs := []string{""}
	genfiles := []string{}

processSubdirs:
	for len(pendingSubDirs) > 0 {
		relSubdir := pendingSubDirs[0]
		pendingSubDirs = pendingSubDirs[1:]
		absSubdir := filepath.Join(dir, relSubdir)
		entries, err := os.ReadDir(absSubdir)
		if err != nil {
			return nil, errors.E(err)
		}

		// We need to skip all other files/dirs if we find a config.SkipFilename
		for _, entry := range entries {
			if entry.Name() == config.SkipFilename {
				continue processSubdirs
			}
		}

		for _, entry := range entries {
			if config.Skip(entry.Name()) {
				continue
			}

			if entry.IsDir() {
				isStack := config.IsStack(cfg.Root(), filepath.Join(absSubdir, entry.Name()))
				if isStack {
					continue
				}

				// We want to keep relative paths to initial dir like:
				// - name
				// - dir/name
				// - dir/sub/name
				// - dir/sub/etc/name
				pendingSubDirs = append(pendingSubDirs,
					filepath.Join(relSubdir, entry.Name()))
				continue
			}

			if !entry.Type().IsRegular() {
				continue
			}

			file := filepath.Join(absSubdir, entry.Name())
			data, err := os.ReadFile(file)
			if err != nil {
				return nil, errors.E(err, "checking if file is generated %q", file)
			}

			if hasGenHCLHeader(string(data)) {
				genfiles = append(genfiles, filepath.ToSlash(
					filepath.Join(relSubdir, entry.Name())))
			}
		}
	}

	return genfiles, nil
}

// DetectOutdated will verify if the given config has outdated code
// and return a list of filenames that are outdated, ordered lexicographically.
func DetectOutdated(root *config.Tree) ([]string, error) {
	stacks, err := stack.LoadAll(root)
	if err != nil {
		return nil, err
	}

	projmeta := stack.NewProjectMetadata(root.RootDir(), stacks)

	outdatedFiles := []string{}
	errs := errors.L()

	for _, stack := range stacks {
		outdated, err := stackOutdated(root, projmeta, stack)
		if err != nil {
			errs.Append(err)
			continue
		}

		// We want results relative to root
		stackRelPath := stack.Path().String()[1:]
		for _, file := range outdated {
			outdatedFiles = append(outdatedFiles,
				path.Join(stackRelPath, file))
		}
	}

	orphanedFiles, err := ListGenFiles(root, root.RootDir())
	if err != nil {
		errs.Append(err)
	}

	if err := errs.AsError(); err != nil {
		return nil, err
	}

	outdatedFiles = append(outdatedFiles, orphanedFiles...)
	sort.Strings(outdatedFiles)
	return outdatedFiles, nil
}

// stackOutdated will verify if a given stack has outdated code and return a list
// of filenames that are outdated, ordered lexicographically.
// If the stack has an invalid configuration it will return an error.
func stackOutdated(root *config.Tree, projmeta project.Metadata, st *stack.S) ([]string, error) {
	logger := log.With().
		Str("action", "generate.stackOutdated").
		Stringer("stack", st).
		Logger()

	ctx, report := stack.LoadStackGlobals(root.Root(), projmeta, st)
	if err := report.AsError(); err != nil {
		return nil, errors.E(err, "checking for outdated code")
	}

	stackpath := st.HostPath()
	cfg, ok := root.Lookup(st.Path())
	if !ok {
		panic("unreachable")
	}
	generated, err := loadStackCodeCfgs(cfg, ctx)
	if err != nil {
		return nil, err
	}

	err = validateGeneratedFiles(cfg, generated)
	if err != nil {
		return nil, err
	}

	genfilesOnFs, err := ListGenFiles(root, st.HostPath())
	if err != nil {
		return nil, errors.E(err, "checking for outdated code")
	}

	logger.Debug().Msgf("generated files detected on fs: %v", genfilesOnFs)

	// We start with the assumption that all gen files on the stack
	// are outdated and then update the outdated files set as we go.
	outdatedFiles := newStringSet(genfilesOnFs...)
	err = updateOutdatedFiles(
		stackpath,
		generated,
		outdatedFiles,
	)
	if err != nil {
		return nil, errors.E(err, "checking for outdated files")
	}

	outdated := outdatedFiles.slice()
	sort.Strings(outdated)
	return outdated, nil
}

func updateOutdatedFiles(
	stackpath string,
	generated []GenFile,
	outdatedFiles *stringSet,
) error {
	logger := log.With().
		Str("action", "generate.updateOutdatedFiles").
		Str("stack", stackpath).
		Logger()

	// So we can properly check blocks with condition false/true in any order
	blocksCondTrue := map[string]struct{}{}

	for _, genfile := range generated {
		logger = logger.With().
			Str("label", genfile.Label()).
			Logger()

		filename := genfile.Label()
		targetpath := filepath.Join(stackpath, filename)

		currentCode, codeFound, err := readFile(targetpath)
		if err != nil {
			return err
		}

		if genfile.Condition() {
			blocksCondTrue[filename] = struct{}{}
		}

		_, prevBlockCondTrue := blocksCondTrue[filename]

		if !codeFound {
			if !genfile.Condition() && !prevBlockCondTrue {
				logger.Debug().Msg("not outdated: condition = false")

				outdatedFiles.remove(filename)
				continue
			}

			logger.Debug().Msg("outdated: condition = true and no code on fs")

			outdatedFiles.add(filename)
			continue
		}

		if !genfile.Condition() {
			if prevBlockCondTrue {
				logger.Debug().Msg("condition = false but other block was true, ignoring")
				continue
			}
			logger.Debug().Msg("outdated: condition = false but code exist on fs")

			outdatedFiles.add(filename)
			continue
		}

		generatedCode := genfile.Header() + genfile.Body()
		if generatedCode != currentCode {
			logger.Debug().Msg("outdated: code on fs differs from generated from config")

			outdatedFiles.add(filename)
		} else {
			logger.Debug().Msg("not outdated: code on fs and generated from config equals")

			outdatedFiles.remove(filename)
		}
	}

	return nil
}

func writeGeneratedCode(target string, genfile GenFile) error {
	logger := log.With().
		Str("action", "writeGeneratedCode()").
		Str("file", target).
		Logger()

	body := genfile.Header() + genfile.Body()

	if genfile.Header() != "" {
		// WHY: some file generation strategies don't provide
		// headers, like generate_file, so we can't detect
		// if we are overwriting a Terramate generated file.
		logger.Trace().Msg("checking file can be written")
		if err := checkFileCanBeOverwritten(target); err != nil {
			return err
		}
	}

	logger.Trace().Msg("creating intermediary dirs")
	if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
		return err
	}

	logger.Trace().Msg("writing file")
	return os.WriteFile(target, []byte(body), 0666)
}

func checkFileCanBeOverwritten(path string) error {
	_, _, err := readGeneratedFile(path)
	return err
}

// readGeneratedFile will read the generated file at the given path.
// It returns an error if it can't read the file or if the file is not
// a Terramate generated file.
//
// The returned boolean indicates if the file exists, so the contents of
// the file + true is returned if a file is found, but if no file is found
// it will return an empty string and false indicating that the file doesn't exist.
func readGeneratedFile(path string) (string, bool, error) {
	logger := log.With().
		Str("action", "readGeneratedCode()").
		Str("path", path).
		Logger()

	logger.Trace().Msg("Get file information.")

	data, found, err := readFile(path)
	if err != nil {
		return "", false, err
	}

	if !found {
		return "", false, nil
	}

	logger.Trace().Msg("Check if file has terramate header.")

	if hasGenHCLHeader(data) {
		return data, true, nil
	}

	return "", false, errors.E(ErrManualCodeExists, "check file %q", path)
}

// readFile will load the file at the given path.
// It returns an error if it can't read the file.
//
// The returned boolean indicates if the file exists, so the contents of
// the file + true is returned if a file is found, but if no file is found
// it will return an empty string and false indicating that the file doesn't exist.
func readFile(path string) (string, bool, error) {
	logger := log.With().
		Str("action", "readFile()").
		Str("path", path).
		Logger()

	logger.Trace().Msg("Get file information.")

	_, err := os.Stat(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return "", false, nil
		}
		return "", false, err
	}

	logger.Trace().Msg("Reading file")

	data, err := os.ReadFile(path)
	if err != nil {
		return "", false, err
	}

	return string(data), true, nil
}

type forEachStackFunc func(dir project.Path, evalctx *eval.Context) dirReport

func forEachDir(cfg *config.Tree, workingDir string, fn forEachStackFunc) Report {
	logger := log.With().
		Str("action", "generate.forEachDir()").
		Str("root", cfg.RootDir()).
		Str("workingDir", workingDir).
		Logger()

	report := Report{}

	logger.Trace().Msg("List stacks.")

	stacks, err := stack.LoadAll(cfg)
	if err != nil {
		report.BootstrapErr = err
		return report
	}

	projmeta := stack.NewProjectMetadata(cfg.RootDir(), stacks)

	projdirs := cfg.Root().Dirs()

	for _, dircfg := range projdirs {
		logger := logger.With().
			Stringer("dir", dircfg.ProjDir()).
			Bool("stack", dircfg.IsStack()).
			Logger()

		if !strings.HasPrefix(dircfg.Dir(), workingDir) {
			logger.Trace().Msg("discarding directory outside working dir")
			continue
		}

		var evalctx *eval.Context
		if dircfg.IsStack() {
			logger.Trace().Msg("Load stack globals.")

			st, err := stack.New(cfg.RootDir(), dircfg.Node)
			if err != nil {
				report.addFailure(dircfg.ProjDir(), err)
			}

			_, globalsReport := stack.LoadStackGlobals(dircfg, projmeta, st)
			if err := globalsReport.AsError(); err != nil {
				report.addFailure(dircfg.ProjDir(), errors.E(ErrLoadingGlobals, err))
				continue
			}

			stackctx := stack.NewEvalCtx(projmeta, st, globalsReport.Globals)
			evalctx = stackctx.Context
		} else {
			continue
			evalctx, err = eval.NewContext(dircfg.Dir())
			if err != nil {
				panic(err)
			}

			evalctx.SetNamespace("terramate", projmeta.ToCtyMap())
		}

		logger.Trace().Msg("Calling dir callback.")

		dirReport := fn(dircfg.ProjDir(), evalctx)
		report.addDirReport(dircfg.ProjDir(), dirReport)
	}

	return report
}

func removeStackGeneratedFiles(
	cfg *config.Tree,
	hostpath string,
	genfiles []GenFile,
) (map[string]string, error) {
	logger := log.With().
		Str("action", "generate.removeStackGeneratedFiles()").
		Str("root", cfg.RootDir()).
		Str("dir", hostpath).
		Logger()

	logger.Trace().Msg("listing generated files")

	removedFiles := map[string]string{}
	files, err := ListGenFiles(cfg, hostpath)
	if err != nil {
		return nil, err
	}

	// WHY: not all Terramate files have headers and can be detected
	// so we use the list of files to be generated to check for these
	// They may or not exist.
	for _, genfile := range genfiles {
		// Files that have header or that are inside the stack dir
		// can be detected by ListGenFiles
		if genfile.Header() == "" {
			files = append(files, genfile.Label())
		}
	}

	logger.Trace().Msg("deleting all Terramate generated files")

	for _, filename := range files {
		logger := logger.With().
			Str("filename", filename).
			Logger()

		logger.Trace().Msg("reading current file before removal")

		path := filepath.Join(hostpath, filename)
		body, err := os.ReadFile(path)
		if err != nil {
			if os.IsNotExist(err) {
				logger.Trace().Msg("ignoring file since it doesn't exist")
				continue
			}
			return nil, errors.E(err, "reading gen file before removal")
		}

		logger.Trace().Msg("removing file")

		if err := os.Remove(path); err != nil {
			return nil, errors.E(err, "removing gen file")
		}

		removedFiles[filename] = string(body)
	}

	return removedFiles, nil
}

func hasGenHCLHeader(code string) bool {
	// When changing headers we need to support old ones (or break).
	// For now keeping them here, to avoid breaks.
	for _, header := range []string{genhcl.Header, genhcl.HeaderV0} {
		if strings.HasPrefix(code, header) {
			return true
		}
	}
	return false
}

func checkGeneratedFilesPaths(cfg *config.Tree, generated []GenFile) error {
	logger := log.With().
		Str("action", "generate.checkGeneratedFilesPaths()").
		Logger()

	logger.Trace().Msg("Checking for invalid paths on generated files.")

	errs := errors.L()

	for _, file := range generated {
		relpath := file.Label()
		if !strings.Contains(relpath, "/") {
			continue
		}

		switch {
		case strings.HasPrefix(relpath, "/"):
			errs.Append(errors.E(ErrInvalidGenBlockLabel,
				file.Range(),
				"%s: starts with /",
				file.Label()))
			continue
		case strings.HasPrefix(relpath, "./"):
			errs.Append(errors.E(ErrInvalidGenBlockLabel,
				file.Range(),
				"%s: starts with ./",
				file.Label()))
			continue
		case strings.Contains(relpath, "../"):
			errs.Append(errors.E(ErrInvalidGenBlockLabel,
				file.Range(),
				"%s: contains ../",
				file.Label()))
			continue
		}

		abspath := filepath.Join(cfg.Dir(), relpath)
		destdir := filepath.Dir(abspath)

		// We need to check that destdir, or any of its parents, is not a symlink or a stack.
		for strings.HasPrefix(destdir, cfg.Dir()) && destdir != cfg.Dir() {
			info, err := os.Lstat(destdir)
			if err != nil {
				if errors.Is(err, fs.ErrNotExist) {
					destdir = filepath.Dir(destdir)
					continue
				}
				errs.Append(errors.E(ErrInvalidGenBlockLabel, err,
					file.Range(),
					"%s: checking if dest dir is a symlink",
					file.Label()))
				break
			}
			if (info.Mode() & fs.ModeSymlink) == fs.ModeSymlink {
				errs.Append(errors.E(ErrInvalidGenBlockLabel, err,
					file.Range(),
					"%s: generates code inside a symlink",
					file.Label()))
				break
			}

			if config.IsStack(cfg.Root(), destdir) {
				errs.Append(errors.E(ErrInvalidGenBlockLabel,
					file.Range(),
					"%s: generates code inside another stack %s",
					file.Label(),
					project.PrjAbsPath(cfg.RootDir(), destdir)))
				break
			}
			destdir = filepath.Dir(destdir)
		}
	}

	return errs.AsError()
}

type stringSet struct {
	vals map[string]struct{}
}

func newStringSet(vals ...string) *stringSet {
	ss := &stringSet{
		vals: map[string]struct{}{},
	}
	for _, v := range vals {
		ss.add(v)
	}
	return ss
}

func (ss *stringSet) remove(val string) {
	delete(ss.vals, val)
}

func (ss *stringSet) add(val string) {
	ss.vals[val] = struct{}{}
}

func (ss *stringSet) slice() []string {
	res := make([]string, 0, len(ss.vals))
	for k := range ss.vals {
		res = append(res, k)
	}
	return res
}

func validateGeneratedFiles(cfg *config.Tree, generated []GenFile) error {
	logger := log.With().
		Str("action", "generate.validateGeneratedFiles()").
		Logger()

	logger.Trace().Msg("validating generated files.")

	genset := map[string]GenFile{}
	for _, file := range generated {
		if other, ok := genset[file.Label()]; ok && file.Condition() {
			return errors.E(ErrConflictingConfig,
				"configs from %q and %q generate a file with same name %q have "+
					"`condition = true`",
				file.Range().Path(),
				other.Range().Path(),
				file.Label(),
			)
		}

		if !file.Condition() {
			continue
		}

		genset[file.Label()] = file
	}

	err := checkGeneratedFilesPaths(cfg, generated)
	if err != nil {
		return err
	}

	logger.Trace().Msg("generated files validated successfully.")
	return nil
}

func loadAsserts(cfg *config.Tree, evalctx *eval.Context) ([]config.Assert, error) {
	asserts := []config.Assert{}
	errs := errors.L()

	for _, assertCfg := range cfg.UpwardAssertions() {
		assert, err := config.EvalAssert(evalctx, assertCfg)
		if err != nil {
			errs.Append(err)
		} else {
			asserts = append(asserts, assert)
		}
	}

	if err := errs.AsError(); err != nil {
		return nil, err
	}

	return asserts, nil
}

func loadStackCodeCfgs(
	tree *config.Tree,
	evalctx *eval.Context,
) ([]GenFile, error) {
	var genfilesConfigs []GenFile

	genStackFiles, err := genfile.LoadStackContext(tree, evalctx)
	if err != nil {
		return nil, err
	}

	genhcls, err := genhcl.Load(tree, evalctx)
	if err != nil {
		return nil, err
	}

	for _, f := range genStackFiles {
		genfilesConfigs = append(genfilesConfigs, f)
	}

	for _, f := range genhcls {
		genfilesConfigs = append(genfilesConfigs, f)
	}

	sort.Slice(genfilesConfigs, func(i, j int) bool {
		return genfilesConfigs[i].Label() < genfilesConfigs[j].Label()
	})

	return genfilesConfigs, nil
}

func cleanupOrphaned(cfg *config.Tree, report Report) Report {
	orphanedGenFiles, err := ListGenFiles(cfg, cfg.RootDir())
	if err != nil {
		report.CleanupErr = err
		return report
	}

	deletedFiles := map[project.Path][]string{}
	deleteFailures := map[project.Path]*errors.List{}

	for _, genfile := range orphanedGenFiles {
		genfileAbspath := filepath.Join(cfg.RootDir(), genfile)
		dir := project.NewPath("/" + filepath.ToSlash(filepath.Dir(genfile)))
		if err := os.Remove(genfileAbspath); err != nil {
			if deleteFailures[dir] == nil {
				deleteFailures[dir] = errors.L()
			}
			deleteFailures[dir].Append(err)
			continue
		}

		filename := filepath.Base(genfile)

		log.Info().
			Stringer("dir", dir).
			Str("file", filename).
			Msg("deleted orphaned file")

		deletedFiles[dir] = append(deletedFiles[dir], filename)
	}

	for failedDir, errs := range deleteFailures {
		delFiles := deletedFiles[failedDir]
		delete(deletedFiles, failedDir)

		report.Failures = append(report.Failures, FailureResult{
			Result: Result{
				Dir:     failedDir,
				Deleted: delFiles,
			},
			Error: errs,
		})
	}

	for dir, deletedFiles := range deletedFiles {
		report.Successes = append(report.Successes, Result{
			Dir:     dir,
			Deleted: deletedFiles,
		})
	}

	report.sort()
	return report
}
