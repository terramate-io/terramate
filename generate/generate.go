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
	"github.com/mineiros-io/terramate/hcl"
	"github.com/mineiros-io/terramate/hcl/eval"
	"github.com/mineiros-io/terramate/hcl/info"
	"github.com/mineiros-io/terramate/project"
	"github.com/mineiros-io/terramate/stack"
	"github.com/rs/zerolog"
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
	// Context is the context of the generate block.
	Context() string
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
	results := make([]LoadResult, len(stacks))

	for i, st := range stacks {
		res := LoadResult{Dir: st.Path()}
		loadres := stack.LoadStackGlobals(cfg, projmeta, st)
		if err := loadres.AsError(); err != nil {
			res.Err = err
			results[i] = res
			continue
		}

		generated, err := loadStackCodeCfgs(cfg, projmeta, st, loadres.Globals)
		if err != nil {
			res.Err = err
			results[i] = res
			continue
		}
		res.Files = generated
		results[i] = res
	}

	for _, dircfg := range cfg.Root().AsList() {
		if dircfg.IsEmptyConfig() || dircfg.IsStack() {
			continue
		}
		res := LoadResult{Dir: dircfg.ProjDir()}
		evalctx, err := eval.NewContext(dircfg.Dir())
		if err != nil {
			res.Err = err
			results = append(results, res)
			continue
		}

		var generated []GenFile
		for _, block := range dircfg.Node.Generate.Files {
			if block.Context != genfile.RootContext {
				continue
			}

			file, err := genfile.Eval(block, evalctx)
			if err != nil {
				res.Err = errors.L(res.Err, err).AsError()
				results = append(results, res)
				continue
			}

			generated = append(generated, file)
		}
		if len(generated) > 0 {
			res.Files = generated
			results = append(results, res)
		}
	}
	return results, nil
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
func Do(cfg *config.Tree, workingDir string) Report {
	stackReport := forEachStack(cfg, workingDir, doStackGeneration)
	rootReport := doRootGeneration(cfg, workingDir)
	report := mergeReports(stackReport, rootReport)
	return cleanupOrphaned(cfg, report)
}

func doStackGeneration(
	cfg *config.Tree,
	projmeta project.Metadata,
	stack *stack.S,
	globals *eval.Object,
) dirReport {
	stackpath := stack.HostPath()
	logger := log.With().
		Str("action", "generate.doStackGeneration()").
		Stringer("stack", stack.Path()).
		Logger()

	report := dirReport{}

	logger.Debug().Msg("generating files")

	asserts, err := loadAsserts(cfg, projmeta, stack, globals)
	if err != nil {
		report.err = err
		return report
	}

	generated, err := loadStackCodeCfgs(cfg, projmeta, stack, globals)
	if err != nil {
		report.err = err
		return report
	}

	for _, gen := range generated {
		asserts = append(asserts, gen.Asserts()...)
	}

	err = handleAsserts(cfg.RootDir(), stack.HostPath(), asserts)
	if err != nil {
		report.err = err
		return report
	}

	errsmap := checkFileConflict(generated)
	if len(errsmap) > 0 {
		errs := errors.L()
		for _, err := range errsmap {
			errs.Append(err)
		}
		report.err = errs.AsError()
		return report
	}

	err = validateStackGeneratedFiles(cfg, stackpath, generated)
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

	removedFiles, err = removeStackGeneratedFiles(cfg, stack.HostPath(), generated)
	if err != nil {
		return failureReport(
			report,
			errors.E(err, "removing old generated files"),
		)
	}

	logger.Debug().Msg("saving generated files")

	for _, file := range generated {
		filename := file.Label()
		path := filepath.Join(stackpath, filename)
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
	}

	for filename := range removedFiles {
		log.Info().
			Stringer("stack", stack.Path()).
			Str("file", filename).
			Msg("deleted file")
		report.addDeletedFile(filename)
	}

	logger.Debug().Msg("finished generating files")
	return report
}

func doRootGeneration(cfg *config.Tree, workingDir string) Report {
	logger := log.With().
		Str("action", "generate.doRootGeneration").
		Str("workingDir", workingDir).
		Logger()

	root := cfg.Root()
	stackConfigs := root.Stacks()
	var stackpaths []project.Path
	for _, stack := range stackConfigs {
		stackpaths = append(stackpaths, stack.ProjDir())
	}

	report := Report{}
	projmeta := project.NewMetadata(root.RootDir(), stackpaths)
	evalctx, err := eval.NewContext(cfg.Dir())
	if err != nil {
		report.BootstrapErr = err
		return report
	}

	evalctx.SetNamespace("terramate", projmeta.ToCtyMap())

	var files []GenFile
	for _, cfg := range cfg.Root().AsList() {
		logger = logger.With().
			Stringer("configDir", cfg.ProjDir()).
			Bool("isEmpty", cfg.IsEmptyConfig()).
			Bool("isStack", cfg.IsStack()).
			Logger()

		if cfg.IsEmptyConfig() || cfg.IsStack() {
			logger.Debug().Msg("ignoring directory")
			continue
		}

		blocks := cfg.Node.Generate.Files
		if len(blocks) == 0 {
			continue
		}

		for _, block := range blocks {
			logger = logger.With().
				Str("generate_file.label", block.Label).
				Str("generate_file.context", block.Context).
				Logger()

			if block.Context != genfile.RootContext {
				logger.Debug().Msg("ignoring block")
				continue
			}

			targetDir := project.NewPath(path.Dir(block.Label))
			err = validateRootGenerateBlock(cfg, block)
			if err != nil {
				report.addFailure(targetDir, err)
				return report
			}

			logger.Debug().Msg("block validated successfully")

			file, err := genfile.Eval(block, evalctx)
			if err != nil {
				report.addFailure(targetDir, err)
				return report
			}

			logger.Debug().Msg("block evaluated successfully")

			files = append(files, file)
		}
	}

	logger.Debug().Msg("checking generate_file.context=root conflicts")

	errsmap := checkFileConflict(files)
	if len(errsmap) > 0 {
		if len(errsmap) > 0 {
			for file, err := range errsmap {
				targetDir := path.Dir(file)
				report.addFailure(project.NewPath(targetDir), err)
			}
			return report
		}
	}

	logger.Debug().Msg("no conflicts found")

	generateRootFiles(cfg, files, &report)
	return report
}

func handleAsserts(rootdir string, dir string, asserts []config.Assert) error {
	logger := log.With().
		Str("action", "generate.handleAsserts()").
		Str("dir", dir).
		Logger()
	errs := errors.L()
	for _, assert := range asserts {
		if !assert.Assertion {
			assertRange := assert.Range
			assertRange.Filename = project.PrjAbsPath(rootdir, assert.Range.Filename).String()
			if assert.Warning {
				log.Warn().
					Stringer("origin", assertRange).
					Str("msg", assert.Message).
					Str("dir", dir).
					Msg("assertion failed")
			} else {
				msg := fmt.Sprintf("%s: %s", assertRange, assert.Message)

				logger.Debug().Msgf("assertion failure detected: %s", msg)

				err := errors.E(ErrAssertion, msg)
				errs.Append(err)
			}
		}
	}
	return errs.AsError()
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
// So calling with dir == rootdir will provide all orphaned files inside a project if
// the root of the project is not a stack.
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
				isStack := config.IsStack(cfg, filepath.Join(absSubdir, entry.Name()))
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
func DetectOutdated(cfg *config.Tree) ([]string, error) {
	logger := log.With().
		Str("action", "generate.DetectOutdated()").
		Logger()

	stacks, err := stack.LoadAll(cfg)
	if err != nil {
		return nil, err
	}

	projmeta := stack.NewProjectMetadata(cfg.RootDir(), stacks)

	outdatedFiles := []string{}
	errs := errors.L()

	logger.Debug().Msg("checking outdated code inside stacks")

	for _, stack := range stacks {
		outdated, err := stackOutdated(cfg, projmeta, stack)
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

	// If the root of the project is a stack then there is no
	// need to check orphaned files. All files are owned by
	// the parent stack or its children.
	if cfg.IsStack() {
		logger.Debug().Msg("project root is stack, no need to check for orphaned files")

		sort.Strings(outdatedFiles)
		return outdatedFiles, nil
	}

	logger.Debug().Msg("checking for orphaned files")

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

// stackOutdated will verify if a given stack has outdated code and return a list
// of filenames that are outdated, ordered lexicographically.
// If the stack has an invalid configuration it will return an error.
func stackOutdated(cfg *config.Tree, projmeta project.Metadata, st *stack.S) ([]string, error) {
	logger := log.With().
		Str("action", "generate.stackOutdated").
		Stringer("stack", st).
		Logger()

	report := stack.LoadStackGlobals(cfg, projmeta, st)
	if err := report.AsError(); err != nil {
		return nil, errors.E(err, "checking for outdated code")
	}

	globals := report.Globals
	stackpath := st.HostPath()

	generated, err := loadStackCodeCfgs(cfg, projmeta, st, globals)
	if err != nil {
		return nil, err
	}

	err = validateStackGeneratedFiles(cfg, st.HostPath(), generated)
	if err != nil {
		return nil, err
	}

	genfilesOnFs, err := ListGenFiles(cfg, st.HostPath())
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

type forEachStackFunc func(
	*config.Tree,
	project.Metadata,
	*stack.S,
	*eval.Object,
) dirReport

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
			report.addFailure(st.Path(), errors.E(ErrLoadingGlobals, err))
			continue
		}

		logger.Trace().Msg("Calling stack callback.")

		stackReport := fn(cfg, projmeta, st, globalsReport.Globals)
		report.addDirReport(st.Path(), stackReport)
	}

	return report
}

func removeStackGeneratedFiles(
	cfg *config.Tree,
	dir string,
	genfiles []GenFile,
) (map[string]string, error) {
	logger := log.With().
		Str("action", "generate.removeStackGeneratedFiles()").
		Str("root", cfg.RootDir()).
		Str("dir", dir).
		Logger()

	logger.Trace().Msg("listing generated files")

	removedFiles := map[string]string{}
	files, err := ListGenFiles(cfg, dir)
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

		path := filepath.Join(dir, filename)
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

func generateRootFiles(
	cfg *config.Tree,
	genfiles []GenFile,
	report *Report,
) {
	logger := log.With().
		Str("action", "generate.generateRootFiles()").
		Logger()

	diskFiles := map[string]string{}       // files already on disk
	mustExistFiles := map[string]GenFile{} // files that must be present on disk
	mustDeleteFiles := map[string]bool{}   // files to be deleted

	// this computes the files that must be present on disk after generate
	// returns. They should not be touched if they already exist on disk and
	// have the same content.
	for _, file := range genfiles {
		if file.Condition() {
			logger := genFileLogger(logger, file)
			logger.Debug().Msg("file must be generated (if needed)")

			mustExistFiles[file.Label()] = file
		}
	}

	// this computes the files that must be deleted if they are present on disk
	for _, file := range genfiles {
		if _, ok := mustExistFiles[file.Label()]; !ok {
			logger := genFileLogger(logger, file)
			logger.Debug().Msg("file must be deleted (if needed)")

			mustDeleteFiles[file.Label()] = true
		}
	}

	// reads the content of must-exist files (if they are present).
	for label := range mustExistFiles {
		logger := logger.With().Str("file", label).Logger()

		logger.Debug().Msg("reading the content of the file on disk")

		abspath := filepath.Join(cfg.RootDir(), label)
		dir := path.Dir(label)
		body, err := os.ReadFile(abspath)
		if err != nil {
			if os.IsNotExist(err) {
				logger.Debug().Msg("file do not exists")

				continue
			}
			dirReport := dirReport{}
			dirReport.err = errors.E(err, "reading generated file")
			report.addDirReport(project.NewPath(dir), dirReport)
			return
		}

		logger.Debug().Msg("file content read successfully")

		diskFiles[label] = string(body)
	}

	// this deletes the files that exist but have condition=false.
	for label, mustDelete := range mustDeleteFiles {
		logger := logger.With().Str("file", label).Logger()

		if mustDelete {
			abspath := filepath.Join(cfg.RootDir(), label)
			_, err := os.Lstat(abspath)
			if err == nil {
				logger.Debug().Msg("deleting file")

				dirReport := dirReport{}
				dir := path.Dir(label)

				err := os.Remove(abspath)
				if err != nil {
					dirReport.err = errors.E(err, "deleting file")
				} else {
					dirReport.addDeletedFile(path.Base(label))
				}
				report.addDirReport(project.NewPath(dir), dirReport)

				logger.Debug().Msg("deleted successfully")
			}
		}
	}

	// this writes the files that must exist (if needed).
	for label, genfile := range mustExistFiles {
		logger := genFileLogger(logger, genfile)

		logger.Debug().Msg("generating file (if needed)")

		abspath := filepath.Join(cfg.RootDir(), label)
		filename := path.Base(label)
		dir := project.NewPath(path.Dir(label))
		body := genfile.Header() + genfile.Body()

		dirReport := dirReport{}
		diskContent, existOnDisk := diskFiles[label]
		if !existOnDisk || body != diskContent {
			logger.Debug().
				Bool("existOnDisk", existOnDisk).
				Bool("fileChanged", body != diskContent).
				Msg("writing file")

			err := writeGeneratedCode(abspath, genfile)
			if err != nil {
				dirReport.err = errors.E(err, "saving file %s", label)
				report.addDirReport(dir, dirReport)
				continue
			}

			logger.Debug().Msg("successfully written")
		}

		if !existOnDisk {
			dirReport.addCreatedFile(filename)
		} else if body != diskContent {
			dirReport.addChangedFile(label)
		} else {
			logger.Debug().Msg("nothing to do, file on disk is up to date.")
		}

		report.addDirReport(dir, dirReport)
	}
}

func genFileLogger(logger zerolog.Logger, genfile GenFile) zerolog.Logger {
	return genBlockLogger(logger, "generate_file", genfile.Label(), genfile.Context())
}

func genFileBlockLogger(logger zerolog.Logger, block hcl.GenFileBlock) zerolog.Logger {
	return genBlockLogger(logger, "generate_file", block.Label, block.Context)
}

func genBlockLogger(logger zerolog.Logger, blockname, label, context string) zerolog.Logger {
	return logger.With().
		Str(fmt.Sprintf("%s.label", blockname), label).
		Str(fmt.Sprintf("%s.context", blockname), context).
		Logger()
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

func validateStackGeneratedFiles(cfg *config.Tree, stackpath string, generated []GenFile) error {
	logger := log.With().
		Str("action", "generate.validateStackGeneratedFiles()").
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

			if config.IsStack(cfg, destdir) {
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

func validateRootGenerateBlock(cfg *config.Tree, block hcl.GenFileBlock) error {
	target := block.Label
	if !path.IsAbs(target) {
		return errors.E(
			ErrInvalidGenBlockLabel, block.Range,
			"%s: is not an absolute path", target,
		)
	}

	abspath := filepath.Join(cfg.RootDir(), filepath.FromSlash(target))
	abspath = filepath.Clean(abspath)
	destdir := filepath.Dir(abspath)

	// We need to check that destdir, or any of its parents, is not a symlink or a stack.
	for strings.HasPrefix(destdir, cfg.RootDir()) && destdir != cfg.RootDir() {
		info, err := os.Lstat(destdir)
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				destdir = filepath.Dir(destdir)
				continue
			}
			return errors.E(
				ErrInvalidGenBlockLabel, err,
				block.Range,
				"%s: checking if dest dir is a symlink",
				target,
			)
		}
		if (info.Mode() & fs.ModeSymlink) == fs.ModeSymlink {
			return errors.E(
				ErrInvalidGenBlockLabel, err,
				block.Range,
				"%s: generates code inside a symlink",
				target,
			)
		}

		if config.IsStack(cfg.Root(), destdir) {
			return errors.E(ErrInvalidGenBlockLabel,
				block.Range,
				"%s: generate_file.context=root generates inside a stack %s",
				target,
				project.PrjAbsPath(cfg.RootDir(), destdir),
			)
		}
		destdir = filepath.Dir(destdir)
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

func checkFileConflict(generated []GenFile) map[string]error {
	genset := map[string]GenFile{}
	errsmap := map[string]error{}
	for _, file := range generated {
		if other, ok := genset[file.Label()]; ok && file.Condition() {
			errsmap[file.Label()] = errors.E(ErrConflictingConfig,
				"configs from %q and %q generate a file with same name %q have "+
					"`condition = true`",
				file.Range().Path(),
				other.Range().Path(),
				file.Label(),
			)
			continue
		}

		if !file.Condition() {
			continue
		}

		genset[file.Label()] = file
	}
	return errsmap
}

func loadAsserts(tree *config.Tree, meta project.Metadata, sm stack.Metadata, globals *eval.Object) ([]config.Assert, error) {
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

func loadStackCodeCfgs(
	tree *config.Tree,
	projmeta project.Metadata,
	st *stack.S,
	globals *eval.Object,
) ([]GenFile, error) {
	var genfilesConfigs []GenFile

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
	logger := log.With().
		Str("action", "generate.cleanupOrphaned()").
		Logger()
	// If the root of the tree is a stack then there is nothing to do
	// since there can't be any orphans (the root parent stack owns
	// the entire project).
	if cfg.IsStack() {
		logger.Debug().Msg("project root is a stack, nothing to do")
		return report
	}

	logger.Debug().Msg("listing orphaned generated files")

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
