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
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/mineiros-io/terramate/config"
	"github.com/mineiros-io/terramate/errors"
	"github.com/mineiros-io/terramate/generate/genfile"
	"github.com/mineiros-io/terramate/generate/genhcl"
	"github.com/mineiros-io/terramate/globals"
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

	// ErrInvalidFilePath indicates that code generation configuration
	// has an invalid filepath as the target to save the generated code.
	ErrInvalidFilePath errors.Kind = "invalid filepath"
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
func Do(rootdir string, workingDir string) Report {
	report := forEachStack(rootdir, workingDir, func(
		projmeta project.Metadata,
		stack *stack.S,
		globals globals.G,
	) dirReport {
		stackpath := stack.HostPath()
		logger := log.With().
			Str("action", "generate.Do()").
			Str("path", rootdir).
			Str("stackpath", stackpath).
			Logger()

		var generated []fileInfo
		report := dirReport{}

		logger.Trace().Msg("generate code from generate_file blocks")

		genfiles, err := genfile.Load(projmeta, stack, globals)
		if err != nil {
			report.err = err
			return report
		}

		logger.Trace().Msg("generate code from generate_hcl blocks")

		genhcls, err := genhcl.Load(projmeta, stack, globals)
		if err != nil {
			report.err = err
			return report
		}

		for _, f := range genfiles {
			generated = append(generated, f)
		}

		for _, f := range genhcls {
			generated = append(generated, f)
		}

		sort.Slice(generated, func(i, j int) bool {
			return generated[i].Name() < generated[j].Name()
		})

		err = validateGeneratedFiles(generated)
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

		removedFiles, err = removeStackGeneratedFiles(stack, generated)
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

			filename := file.Name()
			path := filepath.Join(stackpath, filename)
			logger := logger.With().
				Str("filename", filename).
				Bool("condition", file.Condition()).
				Logger()

			// We don't want to generate files just with a header inside.
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
				report.addCreatedFile(filename)
			} else {
				body := file.Header() + file.Body()
				if body != removedFileBody {
					report.addChangedFile(filename)
				}
				delete(removedFiles, filename)
			}
			logger.Trace().Msg("saved generated file")
		}

		for filename := range removedFiles {
			report.addDeletedFile(filename)
		}
		return report
	})

	outdatedDirs, err := listGenFilesOutsideStacks(rootdir, rootdir)
	if err != nil {
		report.CleanupErr = err
		return report
	}

outdatedDirsLoop:
	for _, outdatedDir := range outdatedDirs {
		relpath := strings.TrimPrefix(outdatedDir.dir, rootdir)
		deleted := []string{}

		for _, outdatedFile := range outdatedDir.files {
			if err := os.Remove(filepath.Join(outdatedDir.dir, outdatedFile)); err != nil {
				report.Failures = append(report.Failures, FailureResult{
					Result: Result{
						Dir:     relpath,
						Deleted: deleted,
					},
					Error: err,
				})
				continue outdatedDirsLoop
			}
			deleted = append(deleted, outdatedFile)
		}

		report.Successes = append(report.Successes, Result{
			Dir:     relpath,
			Deleted: deleted,
		})
	}
	return report
}

// ListGenFiles will list the filenames of all generated code inside
// the given dir.  The dir must be an absolute path.
// The filenames are ordered lexicographically.
func ListGenFiles(dir string) ([]string, error) {
	logger := log.With().
		Str("action", "generate.ListGenFiles()").
		Str("dir", dir).
		Logger()

	logger.Trace().Msg("listing stack dir files")

	dirEntries, err := os.ReadDir(dir)
	if err != nil {
		return nil, errors.E(err, "listing stack files")
	}

	genfiles := []string{}

	for _, dirEntry := range dirEntries {
		logger := logger.With().
			Str("filename", dirEntry.Name()).
			Logger()

		if !dirEntry.Type().IsRegular() {
			logger.Trace().
				Msgf("Ignoring file type: %s", dirEntry.Type())

			continue
		}

		logger.Trace().Msg("Checking if file is generated by terramate")

		filepath := filepath.Join(dir, dirEntry.Name())
		data, err := os.ReadFile(filepath)
		if err != nil {
			return nil, errors.E(err, "checking if file is generated %q", filepath)
		}

		logger.Trace().Msg("File read, checking for terramate headers")

		if hasGenHCLHeader(string(data)) {
			logger.Trace().Msg("Terramate header detected")
			genfiles = append(genfiles, dirEntry.Name())
		}
	}

	logger.Trace().Msg("Done listing stack generated files")
	return genfiles, nil
}

// CheckStack will verify if a given stack has outdated code and return a list
// of filenames that are outdated, ordered lexicographically.
// If the stack has an invalid configuration it will return an error.
//
// The provided root must be the project's root directory as an absolute path.
func CheckStack(projmeta project.Metadata, st *stack.S) ([]string, error) {
	logger := log.With().
		Str("action", "generate.CheckStack()").
		Str("root", projmeta.Rootdir()).
		Stringer("stack", st).
		Logger()

	logger.Trace().Msg("Loading globals for stack.")

	report := stack.LoadStackGlobals(projmeta, st)
	if err := report.AsError(); err != nil {
		return nil, errors.E(err, "checking for outdated code")
	}

	globals := report.Evaluated
	stackpath := st.HostPath()
	var generated []fileInfo

	genfiles, err := genfile.Load(projmeta, st, globals)
	if err != nil {
		return nil, err
	}

	genhcls, err := genhcl.Load(projmeta, st, globals)
	if err != nil {
		return nil, err
	}

	for _, f := range genfiles {
		generated = append(generated, f)
	}

	for _, f := range genhcls {
		generated = append(generated, f)
	}

	err = validateGeneratedFiles(generated)
	if err != nil {
		return nil, err
	}

	logger.Trace().Msg("Listing current generated files.")

	actualGenFiles, err := ListGenFiles(st.HostPath())
	if err != nil {
		return nil, errors.E(err, "checking for outdated code")
	}

	// We start with the assumption that all gen files on the stack
	// are outdated and then update the outdated files set as we go.
	outdatedFiles := newStringSet(actualGenFiles...)
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

type fileInfo interface {
	Name() string
	Origin() string
	Header() string
	Body() string
	Condition() bool
}

func updateOutdatedFiles(
	stackpath string,
	generated []fileInfo,
	outdatedFiles *stringSet,
) error {
	logger := log.With().
		Str("action", "generate.updateOutdatedFiles()").
		Str("stackpath", stackpath).
		Logger()

	logger.Trace().Msg("Checking for outdated generated code on stack.")

	for _, genfile := range generated {
		filename := genfile.Name()
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

func writeGeneratedCode(target string, genfile fileInfo) error {
	logger := log.With().
		Str("action", "writeGeneratedCode()").
		Str("file", target).
		Logger()

	body := genfile.Header() + genfile.Body()

	if genfile.Header() != "" {
		// WHY: some file generation strategies don't provide
		// headers, like generate_file, so we can't detect
		// if we are overwriting a Terramate generated file.
		logger.Trace().Msg("Checking file can be written.")
		if err := checkFileCanBeOverwritten(target); err != nil {
			return err
		}
	}

	logger.Trace().Msg("Writing file")
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

type forEachStackFunc func(project.Metadata, *stack.S, globals.G) dirReport

func forEachStack(root, workingDir string, fn forEachStackFunc) Report {
	logger := log.With().
		Str("action", "generate.forEachStack()").
		Str("root", root).
		Str("workingDir", workingDir).
		Logger()

	report := Report{}

	logger.Trace().Msg("List stacks.")

	stacks, err := stack.LoadAll(root)
	if err != nil {
		report.BootstrapErr = err
		return report
	}

	projmeta := stack.NewProjectMetadata(root, stacks)

	for _, st := range stacks {
		logger := logger.With().
			Stringer("stack", st).
			Logger()

		if !strings.HasPrefix(st.HostPath(), workingDir) {
			logger.Trace().Msg("discarding stack outside working dir")
			continue
		}

		logger.Trace().Msg("Load stack globals.")

		globalsReport := stack.LoadStackGlobals(projmeta, st)
		if err := globalsReport.AsError(); err != nil {
			report.addFailure(st, errors.E(ErrLoadingGlobals, err))
			continue
		}

		logger.Trace().Msg("Calling stack callback.")

		report.addDirReport(st.Path(), fn(projmeta, st, globalsReport.Evaluated))
	}
	report.sortFilenames()
	return report
}

func removeStackGeneratedFiles(stack *stack.S, genfiles []fileInfo) (map[string]string, error) {
	logger := log.With().
		Str("action", "generate.removeStackGeneratedFiles()").
		Stringer("stack", stack).
		Logger()

	logger.Trace().Msg("listing generated files")

	removedFiles := map[string]string{}

	files, err := ListGenFiles(stack.HostPath())
	if err != nil {
		return nil, err
	}

	// WHY: not all Terramate files have headers and can be detected
	// so we use the list of files to be generated to check for these
	// They may or not exist.
	for _, genfile := range genfiles {
		// Files that have header can be detected by ListStackGenFiles
		if genfile.Header() == "" {
			files = append(files, genfile.Name())
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

func checkGeneratedFilesPaths(generated []fileInfo) error {
	logger := log.With().
		Str("action", "checkGeneratedFilesPaths()").
		Logger()

	logger.Trace().Msg("Checking for invalid paths on generated files.")

	for _, file := range generated {
		fname := filepath.ToSlash(file.Name())
		if strings.Contains(fname, "/") {
			return errors.E(
				ErrInvalidFilePath,
				"filenames with dirs are disallowed, config %q provided filename %q",
				file.Origin(),
				file.Name())
		}
	}
	return nil
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

func validateGeneratedFiles(generated []fileInfo) error {
	logger := log.With().
		Str("action", "generate.validateGeneratedFiles()").
		Logger()

	logger.Trace().Msg("validating generated files.")

	genset := map[string]fileInfo{}
	for _, file := range generated {
		if other, ok := genset[file.Name()]; ok && file.Condition() {
			return errors.E(ErrConflictingConfig,
				"configs from %q and %q generate a file with same name %q have "+
					"`condition = true`",
				file.Origin(),
				other.Origin(),
				file.Name(),
			)
		}

		if !file.Condition() {
			continue
		}

		genset[file.Name()] = file
	}

	err := checkGeneratedFilesPaths(generated)
	if err != nil {
		return err
	}

	logger.Trace().Msg("generated files validated successfully.")
	return nil
}

type dirGenFiles struct {
	dir   string
	files []string
}

// listGenFilesOutsideStacks returns a map of dir -> generated files.
func listGenFilesOutsideStacks(rootdir, dir string) ([]dirGenFiles, error) {
	logger := log.With().
		Str("action", "generate.listGenFilesOutsideStacks()").
		Str("dir", dir).
		Logger()

	logger.Trace().Msg("checking if dir is stack")

	dirsFiles := []dirGenFiles{}

	isStack, err := config.IsStack(rootdir, dir)
	if err != nil {
		return nil, errors.E(err, "checking if dir is stack")
	}
	if !isStack {
		logger.Trace().Msg("dir is not stack, checking for generated files")

		genfiles, err := ListGenFiles(dir)
		if err != nil {
			return nil, err
		}

		if len(genfiles) > 0 {
			dirsFiles = append(dirsFiles, dirGenFiles{dir: dir, files: genfiles})
		}
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, errors.E(err, "reading dir to list generated files")
	}

	for _, entry := range entries {
		if entry.IsDir() {
			childPath := filepath.Join(dir, entry.Name())
			childGenFiles, err := listGenFilesOutsideStacks(rootdir, childPath)
			if err != nil {
				return nil, err
			}
			dirsFiles = append(dirsFiles, childGenFiles...)
		}
	}

	return dirsFiles, nil
}
