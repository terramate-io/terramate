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
	mcty "github.com/mineiros-io/terramate/hcl/cty"
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
func Do(cfg *config.Tree, workingDir string) Report {
	report := forEachStack(cfg, workingDir, func(
		projmeta project.Metadata,
		stack *stack.S,
		globals *mcty.Object,
	) dirReport {
		stackpath := stack.HostPath()
		logger := log.With().
			Str("action", "generate.Do()").
			Str("rootdir", cfg.RootDir()).
			Stringer("stack", stack.Path()).
			Logger()

		report := dirReport{}

		logger.Trace().Msg("loading asserts for stack")

		asserts, err := loadAsserts(cfg, projmeta, stack, globals)
		if err != nil {
			report.err = err
			return report
		}

		logger.Trace().Msg("checking stack asserts")
		errs := errors.L()
		for _, assert := range asserts {
			log.Info().
				Stringer("stack", stack.Path()).
				Str("msg", assert.Message).
				Msg("checking assertion")

			if !assert.Assertion {
				assertRange := assert.Range
				assertRange.Filename = project.PrjAbsPath(cfg.RootDir(), assert.Range.Filename).String()
				if assert.Warning {
					log.Warn().
						Stringer("origin", assertRange).
						Str("msg", assert.Message).
						Stringer("stack", stack.Path()).
						Msg("assertion failed")
				} else {
					msg := fmt.Sprintf("%s: %s", assertRange, assert.Message)
					err := errors.E(ErrAssertion, msg)
					errs.Append(err)
				}
			}
		}

		if err := errs.AsError(); err != nil {
			report.err = err
			return report
		}

		generated, err := loadGenCodeConfigs(cfg, projmeta, stack, globals)
		if err != nil {
			report.err = err
			return report
		}

		err = validateGeneratedFiles(cfg, stackpath, generated)
		if err != nil {
			report.err = err
			return report
		}

		logger.Trace().Msg("Removing outdated generated files.")

		var removedFiles map[string]string

		failureReport := func(r dirReport, err error) dirReport {
			r.err = err
			for filename := range removedFiles {
				r.addDeletedFile(filename)
			}
			return r
		}

		removedFiles, err = removeStackGeneratedFiles(cfg, stack, generated)
		if err != nil {
			return failureReport(
				report,
				errors.E(err, "removing old generated files"),
			)
		}

		logger.Trace().Msg("Saving generated files.")

		for _, file := range generated {
			if !file.Condition() {
				continue
			}

			filename := file.Label()
			path := filepath.Join(stackpath, filename)
			logger := logger.With().
				Str("filename", filename).
				Bool("condition", file.Condition()).
				Logger()

			if !file.Condition() {
				logger.Debug().Msg("ignoring")
				continue
			}

			logger.Trace().Msg("saving generated file")

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
					Stringer("stack", stack.Path()).
					Str("file", filename).
					Msg("created file")

				report.addCreatedFile(filename)
			} else {
				body := file.Header() + file.Body()
				if body != removedFileBody {
					log.Info().
						Stringer("stack", stack.Path()).
						Str("file", filename).
						Msg("changed file")

					report.addChangedFile(filename)
				}
				delete(removedFiles, filename)
			}
			logger.Trace().Msg("saved generated file")
		}

		for filename := range removedFiles {
			log.Info().
				Stringer("stack", stack.Path()).
				Str("file", filename).
				Msg("deleted file")
			report.addDeletedFile(filename)
		}
		return report
	})

	return cleanupOrphaned(cfg, report)
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
	logger := log.With().
		Str("action", "generate.ListGenFiles()").
		Str("rootdir", cfg.RootDir()).
		Str("dir", dir).
		Logger()

	logger.Trace().Msg("listing stack dir files")

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

		logger = logger.With().
			Str("dir", relSubdir).
			Logger()

		// We need to skip all other files/dirs if we find a config.SkipFilename
		for _, entry := range entries {
			if entry.Name() == config.SkipFilename {
				logger.Trace().Msg("found skip file: ignoring dir and all its contents")
				continue processSubdirs
			}
		}

		for _, entry := range entries {
			logger := logger.With().
				Str("entry", entry.Name()).
				Str("dir", absSubdir).
				Logger()

			if config.Skip(entry.Name()) {
				logger.Trace().Msg("ignoring file/dir")
				continue
			}

			if entry.IsDir() {
				logger.Trace().Msg("subdir detected, checking if it is stack")

				isStack := config.IsStack(cfg, filepath.Join(absSubdir, entry.Name()))
				if isStack {
					logger.Trace().Msg("ignoring stack subdir")
					continue
				}

				logger.Trace().Msg("not a stack, adding as pending")
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
				logger.Trace().
					Msgf("Ignoring file type: %s", entry.Type())

				continue
			}

			logger.Trace().Msg("Checking if file is generated by terramate")

			file := filepath.Join(absSubdir, entry.Name())
			data, err := os.ReadFile(file)
			if err != nil {
				return nil, errors.E(err, "checking if file is generated %q", file)
			}

			logger.Trace().Msg("File read, checking for terramate headers")

			if hasGenHCLHeader(string(data)) {
				logger.Trace().Msg("Terramate header detected")
				genfiles = append(genfiles, filepath.ToSlash(
					filepath.Join(relSubdir, entry.Name())))
			}
		}
	}

	logger.Trace().Msg("listed generated files with success")
	return genfiles, nil
}

// Check will verify if the given project located at rootdir has outdated code
// and return a list of filenames that are outdated, ordered lexicographically.
// If any directory on the project has an invalid Terramate configuration inside
// it will return an error.
//
// The provided root must be the project's root directory as an absolute path.
func Check(cfg *config.Tree) ([]string, error) {
	logger := log.With().
		Str("action", "generate.Check").
		Str("rootdir", cfg.RootDir()).
		Logger()

	logger.Trace().Msg("loading all stacks")

	stacks, err := stack.LoadAll(cfg)
	if err != nil {
		return nil, err
	}

	logger.Trace().Msg("Checking if any stack has outdated code.")
	projmeta := stack.NewProjectMetadata(cfg.RootDir(), stacks)

	outdatedFiles := []string{}
	errs := errors.L()

	for _, stack := range stacks {
		logger := logger.With().
			Stringer("stack", stack).
			Logger()

		logger.Trace().Msg("checking stack for outdated code")

		outdated, err := CheckStack(cfg, projmeta, stack)
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

	orphanedFiles, err := ListGenFiles(cfg, cfg.RootDir())
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

// CheckStack will verify if a given stack has outdated code and return a list
// of filenames that are outdated, ordered lexicographically.
// If the stack has an invalid configuration it will return an error.
func CheckStack(cfg *config.Tree, projmeta project.Metadata, st *stack.S) ([]string, error) {
	logger := log.With().
		Str("action", "generate.CheckStack()").
		Str("root", projmeta.Rootdir()).
		Stringer("stack", st).
		Logger()

	logger.Trace().Msg("Loading globals for stack.")

	report := stack.LoadStackGlobals(cfg, projmeta, st)
	if err := report.AsError(); err != nil {
		return nil, errors.E(err, "checking for outdated code")
	}

	globals := report.Globals
	stackpath := st.HostPath()

	generated, err := loadGenCodeConfigs(cfg, projmeta, st, globals)
	if err != nil {
		return nil, err
	}

	err = validateGeneratedFiles(cfg, st.HostPath(), generated)
	if err != nil {
		return nil, err
	}

	logger.Trace().Msg("Listing current generated files.")

	genfilesOnFs, err := ListGenFiles(cfg, st.HostPath())
	if err != nil {
		return nil, errors.E(err, "checking for outdated code")
	}

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

type genCodeCfg interface {
	Label() string
	Origin() project.Path
	Header() string
	Body() string
	Condition() bool
}

func updateOutdatedFiles(
	stackpath string,
	generated []genCodeCfg,
	outdatedFiles *stringSet,
) error {
	logger := log.With().
		Str("action", "generate.updateOutdatedFiles()").
		Str("stackpath", stackpath).
		Logger()

	logger.Trace().Msg("Checking for outdated generated code on stack.")

	for _, genfile := range generated {
		filename := genfile.Label()
		targetpath := filepath.Join(stackpath, filename)
		logger := logger.With().
			Str("blockName", filename).
			Str("targetpath", targetpath).
			Logger()

		logger.Trace().Msg("Checking if code is updated.")

		currentCode, codeFound, err := readFile(targetpath)
		if err != nil {
			return err
		}
		if !codeFound {
			if !genfile.Condition() {
				logger.Trace().Msg("Not outdated since file not found and condition is false")
				outdatedFiles.remove(filename)
				continue
			}

			logger.Trace().Msg("outdated since file not found and condition for generation is true")
			outdatedFiles.add(filename)
			continue
		}

		if !genfile.Condition() {
			logger.Trace().Msg("outdated since file exists but condition for generation is false")
			outdatedFiles.add(filename)
			continue
		}

		generatedCode := genfile.Header() + genfile.Body()
		if generatedCode != currentCode {
			logger.Trace().Msg("Generated code doesn't match file, is outdated")
			outdatedFiles.add(filename)
		} else {
			logger.Trace().Msg("Generated code matches file, it is updated")
			outdatedFiles.remove(filename)
		}
	}

	return nil
}

func writeGeneratedCode(target string, genfile genCodeCfg) error {
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

type forEachStackFunc func(project.Metadata, *stack.S, *mcty.Object) dirReport

func forEachStack(cfg *config.Tree, workingDir string, fn forEachStackFunc) Report {
	logger := log.With().
		Str("action", "generate.forEachStack()").
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

	for _, st := range stacks {
		logger := logger.With().
			Stringer("stack", st).
			Logger()

		if !strings.HasPrefix(st.HostPath(), workingDir) {
			logger.Trace().Msg("discarding stack outside working dir")
			continue
		}

		logger.Trace().Msg("Load stack globals.")

		globalsReport := stack.LoadStackGlobals(cfg, projmeta, st)
		if err := globalsReport.AsError(); err != nil {
			report.addFailure(st, errors.E(ErrLoadingGlobals, err))
			continue
		}

		logger.Trace().Msg("Calling stack callback.")

		stackReport := fn(projmeta, st, globalsReport.Globals)
		report.addDirReport(st.Path(), stackReport)
	}

	return report
}

func removeStackGeneratedFiles(
	cfg *config.Tree,
	stack *stack.S,
	genfiles []genCodeCfg,
) (map[string]string, error) {
	logger := log.With().
		Str("action", "generate.removeStackGeneratedFiles()").
		Str("root", cfg.RootDir()).
		Stringer("stack", stack).
		Logger()

	logger.Trace().Msg("listing generated files")

	removedFiles := map[string]string{}
	files, err := ListGenFiles(cfg, stack.HostPath())
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

		path := filepath.Join(stack.HostPath(), filename)
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

func checkGeneratedFilesPaths(cfg *config.Tree, stackpath string, generated []genCodeCfg) error {
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
				"%s: %s: starts with /",
				file.Origin(), file.Label()))
			continue
		case strings.HasPrefix(relpath, "./"):
			errs.Append(errors.E(ErrInvalidGenBlockLabel,
				"%s: %s: starts with ./",
				file.Origin(), file.Label()))
			continue
		case strings.Contains(relpath, "../"):
			errs.Append(errors.E(ErrInvalidGenBlockLabel,
				"%s: %s: contains ../",
				file.Origin(), file.Label()))
			continue
		}

		abspath := filepath.Join(stackpath, relpath)
		destdir := filepath.Dir(abspath)

		// We need to check that destdir, or any of its parents, is not a symlink or a stack.
		for strings.HasPrefix(destdir, stackpath) && destdir != stackpath {
			info, err := os.Lstat(destdir)
			if err != nil {
				if errors.Is(err, fs.ErrNotExist) {
					destdir = filepath.Dir(destdir)
					continue
				}
				errs.Append(errors.E(ErrInvalidGenBlockLabel, err,
					"%s: %s: checking if dest dir is a symlink",
					file.Origin(), file.Label()))
				break
			}
			if (info.Mode() & fs.ModeSymlink) == fs.ModeSymlink {
				errs.Append(errors.E(ErrInvalidGenBlockLabel, err,
					"%s: %s: generates code inside a symlink",
					file.Origin(), file.Label()))
				break
			}

			if config.IsStack(cfg, destdir) {
				errs.Append(errors.E(ErrInvalidGenBlockLabel,
					"%s: %s: generates code inside another stack %s",
					file.Origin(), file.Label(),
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

func validateGeneratedFiles(cfg *config.Tree, stackpath string, generated []genCodeCfg) error {
	logger := log.With().
		Str("action", "generate.validateGeneratedFiles()").
		Logger()

	logger.Trace().Msg("validating generated files.")

	genset := map[string]genCodeCfg{}
	for _, file := range generated {
		if other, ok := genset[file.Label()]; ok && file.Condition() {
			return errors.E(ErrConflictingConfig,
				"configs from %q and %q generate a file with same name %q have "+
					"`condition = true`",
				file.Origin(),
				other.Origin(),
				file.Label(),
			)
		}

		if !file.Condition() {
			continue
		}

		genset[file.Label()] = file
	}

	err := checkGeneratedFilesPaths(cfg, stackpath, generated)
	if err != nil {
		return err
	}

	logger.Trace().Msg("generated files validated successfully.")
	return nil
}

func loadAsserts(tree *config.Tree, meta project.Metadata, sm stack.Metadata, globals *mcty.Object) ([]config.Assert, error) {
	logger := log.With().
		Str("action", "generate.loadAsserts").
		Str("rootdir", tree.RootDir()).
		Str("stack", sm.Path().String()).
		Logger()

	curdir := sm.Path()
	asserts := []config.Assert{}
	errs := errors.L()

	for {
		logger = logger.With().
			Stringer("curdir", curdir).
			Logger()

		evalctx := stack.NewEvalCtx(meta, sm, globals)

		cfg, ok := tree.Lookup(curdir)
		if ok {
			for _, assertCfg := range cfg.Node.Asserts {
				assert, err := config.EvalAssert(evalctx.Context, assertCfg)
				if err != nil {
					errs.Append(err)
				} else {
					asserts = append(asserts, assert)
				}
			}
		}

		if p := curdir.Dir(); p != curdir {
			curdir = p
		} else {
			break
		}
	}

	if err := errs.AsError(); err != nil {
		return nil, err
	}

	return asserts, nil
}

func loadGenCodeConfigs(
	tree *config.Tree,
	projmeta project.Metadata,
	st *stack.S,
	globals *mcty.Object,
) ([]genCodeCfg, error) {
	var genfilesConfigs []genCodeCfg

	genfiles, err := genfile.Load(tree, projmeta, st, globals)
	if err != nil {
		return nil, err
	}

	genhcls, err := genhcl.Load(tree, projmeta, st, globals)
	if err != nil {
		return nil, err
	}

	for _, f := range genfiles {
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
