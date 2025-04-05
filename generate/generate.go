// Copyright 2023 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package generate

import (
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/terramate-io/terramate"
	"github.com/terramate-io/terramate/config"
	"github.com/terramate-io/terramate/errors"
	"github.com/terramate-io/terramate/event"
	"github.com/terramate-io/terramate/generate/genfile"
	"github.com/terramate-io/terramate/generate/genhcl"
	genreport "github.com/terramate-io/terramate/generate/report"
	"github.com/terramate-io/terramate/generate/sharing"
	"github.com/terramate-io/terramate/globals"
	"github.com/terramate-io/terramate/hcl"
	"github.com/terramate-io/terramate/hcl/eval"
	"github.com/terramate-io/terramate/hcl/info"
	"github.com/terramate-io/terramate/project"
	"github.com/terramate-io/terramate/stack"
	"github.com/terramate-io/terramate/stdlib"
	tel "github.com/terramate-io/terramate/ui/tui/telemetry"
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
	// Builtin tells if the generated file is builtin.
	Builtin() bool
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
//
// The given vendorDir is used when calculating the vendor path using tm_vendor
// on the generate blocks.
//
// If a critical error that fails the loading of all results happens it returns
// a non-nil error. In this case the error is not specific to generating code
// for a specific dir.
func Load(root *config.Root, vendorDir project.Path) ([]LoadResult, error) {
	stacks, err := config.LoadAllStacks(root, root.Tree())
	if err != nil {
		return nil, err
	}
	results := make([]LoadResult, len(stacks))

	for i, st := range stacks {
		res := LoadResult{Dir: st.Dir()}
		loadres := globals.ForStack(root, st.Stack)
		if err := loadres.AsError(); err != nil {
			res.Err = err
			results[i] = res
			continue
		}
		cfg, _ := root.Lookup(st.Dir())
		generated, err := loadStackCodeCfgs(root, cfg, vendorDir, nil)
		if err != nil {
			res.Err = errors.E(err, "while loading configs of stack %s", st.Dir())
			results[i] = res
			continue
		}
		res.Files = generated
		results[i] = res
	}

	for _, dircfg := range root.Tree().AsList() {
		if dircfg.IsEmptyConfig() || dircfg.IsStack() {
			continue
		}
		res := LoadResult{Dir: dircfg.Dir()}
		evalctx := eval.NewContext(stdlib.Functions(dircfg.HostDir(), root.Tree().Node.Experiments()))

		var generated []GenFile
		for _, block := range dircfg.Node.Generate.Files {
			if block.Context != genfile.RootContext {
				continue
			}

			file, skip, err := genfile.Eval(block, dircfg, evalctx)
			if err != nil {
				res.Err = errors.L(res.Err, err).AsError()
				results = append(results, res)
				continue
			}

			if skip {
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

// Do will generate code for the entire configuration.
//
// There generation mechanism depend on the generate_* block context attribute:
//
// - context=stack
//
// In this case, for each stack in the project, its blocks are loaded using a
// "Stack Evaluation Context" from the stack directory, checked for conflicts
// and generated. A Stack Evaluation Context contains the Project Metadata,
// Stack Metadata and the Globals hierarchically loaded for the stack.
//
// - context=root
//
// In this case, all of the generate_file blocks with context=root from the
// project are loaded, checked for conflicts, evaluated using a "Root Evaluation
// Context" and generated. A Root Evaluation Context contains just the Project
// Metadata.
//
// The given vendorDir is used when calculating the vendor path using tm_vendor
// on the generate blocks. The vendorRequests channel will be used on tm_vendor
// calls to communicate each vendor request. If the caller is not interested on
// [event.VendorRequest] events just pass a nil channel.
//
// It will return a report including details of which directories succeed and
// failed on code generation, any failure found is added to the report but does
// not abort the overall code generation process, so partial results can be
// obtained and the report needs to be inspected to check.
func Do(
	root *config.Root,
	targetDir project.Path,
	parallel int,
	vendorDir project.Path,
	vendorRequests chan<- event.VendorRequest,
) *genreport.Report {
	logger := log.With().
		Stringer("target_dir", targetDir).
		Logger()

	startTime := time.Now()
	defer func() {
		endTime := time.Now()
		logger.Debug().
			Time("started_at", startTime).
			Time("finished_at", endTime).
			Dur("elapsed_time_ms", endTime.Sub(startTime)).
			Msg("generate finished")
	}()

	tree, ok := root.Lookup(targetDir)
	if !ok {
		return &genreport.Report{
			BootstrapErr: errors.E("directory %s not found", targetDir),
		}
	}
	if parallel == 0 {
		parallel = runtime.NumCPU()
	}

	logger = logger.With().Int("parallel", parallel).Logger()

	workchan := make(chan *config.Tree)
	reportchan := make(chan *genreport.Report)
	var wg sync.WaitGroup
	for i := 0; i < parallel; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for cfg := range workchan {
				reportchan <- stackGenerate(root, cfg, vendorDir, vendorRequests)
			}
		}()
	}

	wg.Add(1)
	go func() {
		defer wg.Done()
		reportchan <- rootGenerate(root, targetDir)
	}()

	var report *genreport.Report
	mergedReports := make(chan struct{})
	go func() {
		report = genreport.Merge(reportchan)
		mergedReports <- struct{}{}
	}()

	for _, cfg := range tree.Stacks() {
		workchan <- cfg
	}

	close(workchan)
	wg.Wait()
	close(reportchan)

	<-mergedReports

	return cleanupOrphaned(root, tree, report)
}

// stackGenerate assumes cfg is a stack.
func stackGenerate(
	root *config.Root,
	cfg *config.Tree,
	vendorDir project.Path,
	vendorRequests chan<- event.VendorRequest,
) *genreport.Report {
	logger := log.With().
		Str("action", "stackGenerate()").
		Stringer("stack", cfg.Dir()).
		Logger()

	startTime := time.Now()
	defer func() {
		endTime := time.Now()
		logger.Debug().
			Time("started_at", startTime).
			Time("finished_at", endTime).
			Dur("elapsed_time_ms", endTime.Sub(startTime)).
			Msg("stack generation finished")
	}()
	report := &genreport.Report{}

	_, err := cfg.Stack()
	if err != nil {
		report.BootstrapErr = err
		return report
	}

	generated, err := loadStackCodeCfgs(root, cfg, vendorDir, vendorRequests)
	if err != nil {
		report.AddFailure(cfg.Dir(), err)
		return report
	}

	errsmap := checkFileConflict(generated)
	if len(errsmap) > 0 {
		errs := errors.L()
		for _, err := range errsmap {
			errs.Append(err)
		}
		report.AddFailure(cfg.Dir(), errs.AsError())
		return report
	}

	err = validateStackGeneratedFiles(root, cfg.HostDir(), generated)
	if err != nil {
		report.AddFailure(cfg.Dir(), err)
		return report
	}

	allFiles, err := allStackGeneratedFiles(root, cfg.HostDir(), generated)
	if err != nil {
		report.AddFailure(cfg.Dir(), errors.E(err, "listing all generated files"))
		return report
	}

	logger.Trace().Msg("saving generated files")

	stackReport := genreport.Dir{}

	for _, file := range generated {
		filename := file.Label()
		path := filepath.Join(cfg.HostDir(), filename)
		logger := logger.With().
			Str("filename", filename).
			Logger()

		if !file.Condition() {
			logger.Trace().Msg("condition is false, ignoring file")
			continue
		}

		body := file.Header() + file.Body()

		// Change detection + remove entries that got re-generated
		oldFileBody, oldExists := allFiles[filename]

		if !oldExists || oldFileBody != body {
			err := writeGeneratedCode(root, path, file)
			if err != nil {
				report.AddFailure(cfg.Dir(), errors.E(err, "saving file %q", filename))
				continue
			}
		}

		if !oldExists {
			log.Info().
				Stringer("stack", cfg.Dir()).
				Str("file", filename).
				Msg("created file")

			stackReport.AddCreatedFile(filename)
		} else {
			delete(allFiles, filename)
			if body != oldFileBody {
				log.Info().
					Stringer("stack", cfg.Dir()).
					Str("file", filename).
					Msg("changed file")

				stackReport.AddChangedFile(filename)
			}
		}
	}

	for filename := range allFiles {
		log.Info().
			Stringer("stack", cfg.Dir()).
			Str("file", filename).
			Msg("deleted file")

		stackReport.AddDeletedFile(filename)

		path := filepath.Join(cfg.HostDir(), filename)
		err = os.Remove(path)
		if err != nil {
			report.AddFailure(cfg.Dir(), errors.E("removing file %s", filename))
			continue
		}

		delete(allFiles, filename)
	}

	report.AddDirReport(cfg.Dir(), stackReport)
	return report
}

func rootGenerate(root *config.Root, target project.Path) *genreport.Report {
	logger := log.With().
		Str("action", "rootGenerate()").
		Stringer("target_dir", target).
		Logger()

	startTime := time.Now()
	defer func() {
		endTime := time.Now()
		logger.Debug().
			Time("started_at", startTime).
			Time("finished_at", endTime).
			Dur("elapsed_time_ms", endTime.Sub(startTime)).
			Msg("root generation finished")
	}()

	report := &genreport.Report{}
	evalctx := eval.NewContext(stdlib.Functions(root.HostDir(), root.Tree().Node.Experiments()))
	evalctx.SetNamespace("terramate", root.Runtime())

	var files []GenFile

	for _, cfg := range root.Tree().AsList() {
		logger := logger.With().
			Stringer("configDir", cfg.Dir()).
			Bool("isEmpty", cfg.IsEmptyConfig()).
			Logger()

		if cfg.IsEmptyConfig() || len(cfg.Node.Generate.Files) == 0 {
			logger.Trace().Msg("ignoring directory")
			continue
		}

		blocks := cfg.Node.Generate.Files
		for _, block := range blocks {
			logger := genFileBlockLogger(logger, block)

			if block.Context != genfile.RootContext {
				logger.Trace().Msg("ignoring block")
				continue
			}

			// TODO(i4k): generate report must be redesigned for context=root
			// Here we use path.Clean("/"+path.Dir(label)) to ensure the
			// report.Dir is always absolute.
			targetDir := project.NewPath(path.Clean("/" + path.Dir(block.Label)))
			err := validateRootGenerateBlock(root, block)
			if err != nil {
				report.AddFailure(targetDir, err)
				return report
			}

			logger.Trace().Msg("block validated successfully")

			if !targetDir.HasPrefix(target.String()) {
				logger.Trace().Msg("block out of scope of current generation")
				continue
			}

			file, skip, err := genfile.Eval(block, cfg, evalctx)
			if err != nil {
				report.AddFailure(targetDir, err)
				return report
			}

			if skip {
				continue
			}

			logger.Trace().Msg("block evaluated successfully")

			files = append(files, file)
		}
	}

	logger.Trace().Msg("checking generate_file.context=root conflicts")

	errsmap := checkFileConflict(files)
	if len(errsmap) > 0 {
		if len(errsmap) > 0 {
			for file, err := range errsmap {
				targetDir := path.Dir(file)
				report.AddFailure(project.NewPath(targetDir), err)
			}
			return report
		}
	}

	logger.Trace().Msg("no conflicts found")

	generateRootFiles(root, files, report)
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

// ListStackGenFiles will list the path of all generated code inside the given dir
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
func ListStackGenFiles(root *config.Root, dir string) ([]string, error) {
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
			if entry.Name() == terramate.SkipFilename {
				continue processSubdirs
			}
		}

		for _, entry := range entries {
			if entry.IsDir() {
				// only dotdirs are ignored.
				if entry.Name()[0] == '.' {
					continue
				}

				isStack := config.IsStack(root, filepath.Join(absSubdir, entry.Name()))
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

			if hasGenHCLHeader(genhcl.CommentStyleFromConfig(root.Tree()), string(data)) {
				genfiles = append(genfiles, filepath.ToSlash(
					filepath.Join(relSubdir, entry.Name())))
			}
		}
	}

	return genfiles, nil
}

// DetectOutdated will verify if the given config has outdated code in the target tree
// and return a list of filenames that are outdated, ordered lexicographically.
func DetectOutdated(root *config.Root, target *config.Tree, vendorDir project.Path) ([]string, error) {
	logger := log.With().
		Str("action", "generate.DetectOutdated()").
		Stringer("dir", target.Dir()).
		Logger()

	outdatedFiles := newStringSet()
	errs := errors.L()

	logger.Debug().Msg("checking outdated code inside stacks")

	for _, cfg := range target.Stacks() {
		outdated, err := stackContextOutdated(root, cfg, vendorDir)
		if err != nil {
			errs.Append(err)
			continue
		}

		// We want results relative to root
		dirRelPath := cfg.Dir().String()[1:]
		for _, file := range outdated {
			outdatedFiles.add(path.Join(dirRelPath, file))
		}
	}

	for _, cfg := range target.AsList() {
		outdated, err := rootContextOutdated(root, cfg)
		if err != nil {
			errs.Append(err)
			continue
		}

		// We want results relative to root
		dirRelPath := cfg.Dir().String()[1:]
		for _, file := range outdated {
			outdatedFiles.add(path.Join(dirRelPath, file))
		}
	}

	if err := errs.AsError(); err != nil {
		return nil, err
	}

	// If the base dir is a stack then there is no need to check orphaned files.
	// All files are owned by the parent stack or its children.
	if target.IsStack() || target.IsInsideStack() {
		logger.Debug().Msg("target directory is stack or inside a stack, no need to check for orphaned files")

		outdated := outdatedFiles.slice()
		sort.Strings(outdated)
		return outdated, nil
	}

	logger.Debug().Msg("checking for orphaned files")

	orphanedFiles, err := ListStackGenFiles(root, target.HostDir())
	if err != nil {
		errs.Append(err)
	}

	if err := errs.AsError(); err != nil {
		return nil, err
	}

	// We want results relative to root
	dirRelPath := target.Dir().String()[1:]
	for _, file := range orphanedFiles {
		outdatedFiles.add(path.Join(dirRelPath, file))
	}

	outdated := outdatedFiles.slice()
	sort.Strings(outdated)
	return outdated, nil
}

// stackContextOutdated will verify if a given directory has outdated code
// for blocks with context=stack and return a list of filenames that are outdated.
func stackContextOutdated(root *config.Root, cfg *config.Tree, vendorDir project.Path) ([]string, error) {
	logger := log.With().
		Str("action", "generate.stackOutdated").
		Stringer("stack", cfg.Dir()).
		Logger()

	cfgpath := cfg.HostDir()

	generated, err := loadStackCodeCfgs(root, cfg, vendorDir, nil)
	if err != nil {
		return nil, err
	}
	err = validateStackGeneratedFiles(root, cfgpath, generated)
	if err != nil {
		return nil, err
	}

	genfilesOnFs, err := ListStackGenFiles(root, cfgpath)
	if err != nil {
		return nil, errors.E(err, "checking for outdated code")
	}

	logger.Debug().Msgf("generated files detected on fs: %v", genfilesOnFs)

	// We start with the assumption that all gen files on the stack
	// are outdated and then update the outdated files set as we go.
	outdatedFiles := newStringSet(genfilesOnFs...)
	err = updateOutdatedFiles(root, cfgpath, generated, outdatedFiles)
	if err != nil {
		return nil, errors.E(err, "handling detected files")
	}
	return outdatedFiles.slice(), nil
}

// rootContextOutdated will verify if the given directory has outdated code for context=root blocks
// and return the list of outdated files.
func rootContextOutdated(root *config.Root, cfg *config.Tree) ([]string, error) {
	cfgpath := cfg.HostDir()

	generated, err := loadRootCodeCfgs(root, cfg)
	if err != nil {
		return nil, err
	}

	// We start with the assumption that all gen files on the stack
	// are outdated and then update the outdated files set as we go.
	outdatedFiles := newStringSet()
	err = updateOutdatedFiles(root, cfgpath, generated, outdatedFiles)
	if err != nil {
		return nil, errors.E(err, "handling detected files")
	}
	return outdatedFiles.slice(), nil
}

func updateOutdatedFiles(root *config.Root, cfgpath string, generated []GenFile, outdatedFiles *stringSet) error {
	logger := log.With().
		Str("action", "generate.updateOutdatedFiles").
		Str("stack", cfgpath).
		Logger()

	// So we can properly check blocks with condition false/true in any order
	blocksCondTrue := map[string]struct{}{}

	for _, genfile := range generated {
		logger = logger.With().
			Str("label", genfile.Label()).
			Logger()

		var targetpath string
		var filename string
		if genfile.Context() == "root" {
			filename = genfile.Label()[1:]
			targetpath = filepath.Join(root.HostDir(), filename)
		} else {
			filename = genfile.Label()
			targetpath = filepath.Join(cfgpath, filename)
		}

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

func writeGeneratedCode(root *config.Root, target string, genfile GenFile) error {
	body := genfile.Header() + genfile.Body()

	if genfile.Header() != "" {
		// WHY: some file generation strategies don't provide
		// headers, like generate_file, so we can't detect
		// if we are overwriting a Terramate generated file.
		if err := checkFileCanBeOverwritten(root, target); err != nil {
			return err
		}
	}

	if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
		return err
	}

	return os.WriteFile(target, []byte(body), 0666)
}

func checkFileCanBeOverwritten(root *config.Root, path string) error {
	_, _, err := readGeneratedFile(root, path)
	return err
}

// readGeneratedFile will read the generated file at the given path.
// It returns an error if it can't read the file or if the file is not
// a Terramate generated file.
//
// The returned boolean indicates if the file exists, so the contents of
// the file + true is returned if a file is found, but if no file is found
// it will return an empty string and false indicating that the file doesn't exist.
func readGeneratedFile(root *config.Root, path string) (string, bool, error) {
	data, found, err := readFile(path)
	if err != nil {
		return "", false, err
	}

	if !found {
		return "", false, nil
	}

	if hasGenHCLHeader(genhcl.CommentStyleFromConfig(root.Tree()), data) {
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
	_, err := os.Stat(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return "", false, nil
		}
		return "", false, err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return "", false, err
	}

	return string(data), true, nil
}

func allStackGeneratedFiles(
	root *config.Root,
	dir string,
	genfiles []GenFile,
) (map[string]string, error) {
	allFiles := map[string]string{}
	files, err := ListStackGenFiles(root, dir)
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

	for _, filename := range files {
		path := filepath.Join(dir, filename)
		body, err := os.ReadFile(path)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, errors.E(err, "reading generated file")
		}

		allFiles[filename] = string(body)
	}

	return allFiles, nil
}

func generateRootFiles(root *config.Root, genfiles []GenFile, report *genreport.Report) {
	logger := log.With().
		Str("action", "generate.generateRootFiles()").
		Logger()

	diskFiles := map[string]string{}         // files already on disk
	mustExistFiles := map[string]GenFile{}   // files that must be present on disk
	mustDeleteFiles := map[string]struct{}{} // files to be deleted

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

			mustDeleteFiles[file.Label()] = struct{}{}
		}
	}

	// reads the content of must-exist files (if they are present).
	for label := range mustExistFiles {
		logger := logger.With().Str("file", label).Logger()

		logger.Debug().Msg("reading the content of the file on disk")

		abspath := filepath.Join(root.HostDir(), label)
		dir := path.Dir(label)
		body, err := os.ReadFile(abspath)
		if err != nil {
			if os.IsNotExist(err) {
				logger.Debug().Msg("file do not exists")

				continue
			}
			dirReport := genreport.Dir{}
			dirReport.Err = errors.E(err, "reading generated file")
			report.AddDirReport(project.NewPath(dir), dirReport)
			return
		}

		logger.Debug().Msg("file content read successfully")

		diskFiles[label] = string(body)
	}

	// this deletes the files that exist but have condition=false.
	for label := range mustDeleteFiles {
		logger := logger.With().Str("file", label).Logger()

		abspath := filepath.Join(root.HostDir(), label)
		_, err := os.Lstat(abspath)
		if err == nil {
			logger.Debug().Msg("deleting file")

			dirReport := genreport.Dir{}
			dir := path.Dir(label)

			err := os.Remove(abspath)
			if err != nil {
				dirReport.Err = errors.E(err, "deleting file")
			} else {
				dirReport.AddDeletedFile(path.Base(label))
			}
			report.AddDirReport(project.NewPath(dir), dirReport)

			logger.Debug().Msg("deleted successfully")
		}
	}

	// this writes the files that must exist (if needed).
	for label, genfile := range mustExistFiles {
		logger := genFileLogger(logger, genfile)

		logger.Debug().Msg("generating file (if needed)")

		abspath := filepath.Join(root.HostDir(), label)
		filename := path.Base(label)
		dir := project.NewPath(path.Dir(label))
		body := genfile.Header() + genfile.Body()

		dirReport := genreport.Dir{}
		diskContent, existOnDisk := diskFiles[label]
		if !existOnDisk || body != diskContent {
			logger.Debug().
				Bool("existOnDisk", existOnDisk).
				Bool("fileChanged", body != diskContent).
				Msg("writing file")

			err := writeGeneratedCode(root, abspath, genfile)
			if err != nil {
				dirReport.Err = errors.E(err, "saving file %s", label)
				report.AddDirReport(dir, dirReport)
				continue
			}

			logger.Debug().Msg("successfully written")
		}

		if !existOnDisk {
			dirReport.AddCreatedFile(filename)
		} else if body != diskContent {
			dirReport.AddChangedFile(label)
		} else {
			logger.Debug().Msg("nothing to do, file on disk is up to date.")
		}

		report.AddDirReport(dir, dirReport)
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

func hasGenHCLHeader(commentStyle genhcl.CommentStyle, code string) bool {
	// When changing headers we need to support old ones (or break).
	// For now keeping them here, to avoid breaks.
	for _, header := range []string{genhcl.Header(commentStyle), genhcl.HeaderV0} {
		if strings.HasPrefix(code, header) {
			return true
		}
	}
	return false
}

func validateStackGeneratedFiles(root *config.Root, stackpath string, generated []GenFile) error {
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

		// We need to check that destdir, or any of its parents, is not a symlink, a stack or a dotdir.
		for strings.HasPrefix(destdir, stackpath) && destdir != stackpath {
			dirname := filepath.Base(destdir)
			if dirname[0] == '.' {
				errs.Append(errors.E(
					ErrInvalidGenBlockLabel,
					file.Range(),
					"%s: generation inside dot directories are disallowed",
					file.Label(),
				))
			}
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
				errs.Append(errors.E(ErrInvalidGenBlockLabel,
					file.Range(),
					"%s: generates code inside a symlink",
					file.Label()))
				break
			}

			if config.IsStack(root, destdir) {
				errs.Append(errors.E(ErrInvalidGenBlockLabel,
					file.Range(),
					"%s: generates code inside another stack %s",
					file.Label(),
					project.PrjAbsPath(root.HostDir(), destdir)))
				break
			}
			destdir = filepath.Dir(destdir)
		}
	}

	return errs.AsError()
}

func validateRootGenerateBlock(root *config.Root, block hcl.GenFileBlock) error {
	target := block.Label
	if !path.IsAbs(target) {
		return errors.E(
			ErrInvalidGenBlockLabel, block.Range,
			"%s: is not an absolute path", target,
		)
	}

	abspath := filepath.Join(root.HostDir(), filepath.FromSlash(target))
	abspath = filepath.Clean(abspath)
	destdir := filepath.Dir(abspath)

	if !strings.HasPrefix(destdir, root.HostDir()) {
		return errors.E(ErrInvalidGenBlockLabel,
			"label path computes to %s which is not inside rootdir %s",
			abspath, root.HostDir())
	}

	// We need to check that destdir, or any of its parents, is not a symlink or a stack.
	for strings.HasPrefix(destdir, root.HostDir()) && destdir != root.HostDir() {
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

		if config.IsStack(root, destdir) {
			return errors.E(ErrInvalidGenBlockLabel,
				block.Range,
				"%s: generate_file.context=root generates inside a stack %s",
				target,
				project.PrjAbsPath(root.HostDir(), destdir),
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

func loadAsserts(root *config.Root, st *config.Stack, evalctx *eval.Context) ([]config.Assert, error) {
	logger := log.With().
		Str("action", "generate.loadAsserts").
		Str("rootdir", root.HostDir()).
		Str("stack", st.Dir.String()).
		Logger()

	curdir := st.Dir
	asserts := []config.Assert{}
	errs := errors.L()

	for {
		logger = logger.With().
			Stringer("curdir", curdir).
			Logger()

		cfg, ok := root.Lookup(curdir)
		if ok {
			for _, assertCfg := range cfg.Node.Asserts {
				assert, err := config.EvalAssert(evalctx, assertCfg)
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

func loadRootCodeCfgs(root *config.Root, cfg *config.Tree) ([]GenFile, error) {
	blocks := cfg.Node.Generate.Files

	genfiles := []GenFile{}
	for _, block := range blocks {
		if block.Context != "root" {
			continue
		}
		err := validateRootGenerateBlock(root, block)
		if err != nil {
			return nil, err
		}

		evalctx := eval.NewContext(stdlib.Functions(cfg.RootDir(), root.Tree().Node.Experiments()))
		evalctx.SetNamespace("terramate", root.Runtime())

		file, skip, err := genfile.Eval(block, cfg, evalctx)
		if err != nil {
			return nil, err
		}
		if skip {
			continue
		}
		genfiles = append(genfiles, file)
	}
	return genfiles, nil
}

func loadStackCodeCfgs(
	root *config.Root,
	cfg *config.Tree,
	vendorDir project.Path,
	vendorRequests chan<- event.VendorRequest,
) ([]GenFile, error) {
	st, err := cfg.Stack()
	if err != nil {
		return nil, err
	}
	globals := globals.ForStack(root, st)
	if err := globals.AsError(); err != nil {
		return nil, err
	}
	evalctx := stack.NewEvalCtx(root, st, globals.Globals)
	asserts, err := loadAsserts(root, st, evalctx.Context)
	if err != nil {
		return nil, err
	}

	tel.DefaultRecord.Set(
		tel.BoolFlag("asserts", len(asserts) > 0),
	)

	var genfilesConfigs []GenFile

	genfiles, err := genfile.Load(root, st, evalctx.Context, vendorDir, vendorRequests)
	if err != nil {
		return nil, err
	}

	genhcls, err := genhcl.Load(root, st, evalctx.Context, vendorDir, vendorRequests)
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

	for _, gen := range genfilesConfigs {
		asserts = append(asserts, gen.Asserts()...)
	}

	err = handleAsserts(root.HostDir(), st.HostDir(root), asserts)
	if err != nil {
		return nil, err
	}

	type backendFile struct {
		inputs  config.Inputs
		outputs config.Outputs
	}

	backendMap := map[string]*backendFile{}
	for _, outputBlock := range cfg.Node.Outputs {
		output, err := config.EvalOutput(evalctx.Context, outputBlock)
		if err != nil {
			return nil, err
		}
		v, ok := backendMap[output.Backend]
		if !ok {
			v = &backendFile{}
		}
		v.outputs = append(v.outputs, output)
		backendMap[output.Backend] = v
	}
	for _, inputBlock := range cfg.Node.Inputs {
		input, err := config.EvalInput(evalctx.Context, inputBlock)
		if err != nil {
			return nil, err
		}
		v, ok := backendMap[input.Backend]
		if !ok {
			v = &backendFile{}
		}
		v.inputs = append(v.inputs, input)
		backendMap[input.Backend] = v
	}
	for backendName, file := range backendMap {
		backend, ok := cfg.SharingBackend(backendName)
		if !ok {
			return nil, errors.E("backend %s not found", backendName)
		}
		sharingFile, err := sharing.PrepareFile(root, backend.Filename, file.inputs, file.outputs)
		if err != nil {
			return nil, err
		}
		genfilesConfigs = append(genfilesConfigs, sharingFile)
	}
	return genfilesConfigs, nil
}

func cleanupOrphaned(root *config.Root, target *config.Tree, report *genreport.Report) *genreport.Report {
	logger := log.With().
		Str("action", "generate.cleanupOrphaned()").
		Stringer("dir", target.Dir()).
		Logger()

	defer report.Sort()

	// If the target tree is a stack then there is nothing to do
	// as it was already generated at this point.
	if target.IsStack() {
		return report
	}

	logger.Debug().Msg("listing orphaned generated files")

	orphanedGenFiles, err := ListStackGenFiles(root, target.HostDir())
	if err != nil {
		report.CleanupErr = err
		return report
	}

	deletedFiles := map[project.Path][]string{}
	deleteFailures := map[project.Path]*errors.List{}

	for _, genfile := range orphanedGenFiles {
		genfileAbspath := filepath.Join(target.HostDir(), genfile)
		dir := project.PrjAbsPath(root.HostDir(), filepath.Dir(genfileAbspath))
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

		report.Failures = append(report.Failures, genreport.FailureResult{
			Result: genreport.Result{
				Dir:     failedDir,
				Deleted: delFiles,
			},
			Error: errs,
		})
	}

	for dir, deletedFiles := range deletedFiles {
		report.Successes = append(report.Successes, genreport.Result{
			Dir:     dir,
			Deleted: deletedFiles,
		})
	}
	return report
}
