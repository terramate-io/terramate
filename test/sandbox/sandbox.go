// Copyright 2023 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

// Package sandbox provides an easy way to setup isolated terramate projects
// that can be used on testing, acting like sandboxes.
//
// It helps with:
//
// - git initialization/operations
// - Terraform module creation
// - Terramate stack creation
package sandbox

import (
	"bytes"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/madlambda/spells/assert"
	"github.com/terramate-io/terramate/config"
	"github.com/terramate-io/terramate/generate"
	"github.com/terramate-io/terramate/globals"
	"github.com/terramate-io/terramate/hcl"
	"github.com/terramate-io/terramate/hcl/eval"
	"github.com/terramate-io/terramate/project"
	"github.com/terramate-io/terramate/stack"
	"github.com/terramate-io/terramate/test"
)

// S is a full sandbox with its own base dir that is an initialized git repo for
// test purposes.
type S struct {
	t       testing.TB
	git     *Git
	rootdir string
	cfg     *config.Root
}

// DirEntry represents a directory and can be used to create files inside the
// directory.
type DirEntry struct {
	t        testing.TB
	rootpath string
	abspath  string
	relpath  string
}

// StackEntry represents a directory that's also a stack.
// It extends a DirEntry with stack specific functionality.
type StackEntry struct {
	DirEntry
}

// FileEntry represents a file and can be used to manipulate the file contents.
// It is optimized for reading/writing all contents, not stream programming
// (io.Reader/io.Writer).
// It has limited usefulness but it is easier to work with for testing.
type FileEntry struct {
	t        testing.TB
	hostpath string
	rootpath string
}

// New creates a new complete test sandbox.
// The git repository is set up with sane defaults.
//
// It is a programming error to use a test env created with a testing.TB other
// than the one of the test using the test env, for a new test/sub-test always
// create a new test env for it.
func New(t testing.TB) S {
	s := NoGit(t)

	s.git = NewGit(t, s.RootDir())
	s.git.Init()
	return s
}

// NewWithGitConfig creates a new sandbox using the cfg configuration for the
// git repository.
func NewWithGitConfig(t testing.TB, cfg GitConfig) S {
	s := NoGit(t)

	cfg.repoDir = s.RootDir()

	s.git = NewGitWithConfig(t, cfg)
	s.git.Init()
	return s
}

// NoGit creates a new test sandbox with no git repository.
//
// It is a programming error to use a test env created with a testing.TB other
// than the one of the test using the test env, for a new test/sub-test always
// create a new test env for it.
func NoGit(t testing.TB) S {
	t.Helper()

	// here we create some stacks outside the root directory of the
	// sandbox so we can check if terramate does not ascend to parent
	// directories.

	outerDir := t.TempDir()

	buildTree(t, config.NewRoot(config.NewTree(outerDir)), []string{
		"s:this-stack-must-never-be-visible",
		"s:other-hidden-stack",
	})

	rootdir := filepath.Join(outerDir, "sandbox")
	test.MkdirAll(t, rootdir)

	return S{
		t:       t,
		rootdir: test.CanonPath(t, rootdir),
	}
}

// BuildTree builds a tree layout based on the layout specification.
// Each string in the slice represents a filesystem operation, and each
// operation has the format below:
//
//	<kind>:<relative path>[:param]
//
// Where kind is one of the below:
//
//	"d" for directory creation.
//	"g" for local git directory creation.
//	"s" for initialized stacks.
//	"f" for file creation.
//	"l" for symbolic link creation.
//	"t" for terramate block.
//
// And [:param] is optional and it depends on the command.
//
// For the operations "f" and "s" [:param] is defined as:
//
//	For "f" it is the content of the file to be created.
//	For "s" it is a key value pair of the form:
//	  <attr1>=<val1>[;<attr2>=<val2>]
//
// Where attrN is a string attribute of the terramate block of the stack.
// TODO(i4k): document empty data field.
//
// Example:
//
//	s:name-of-the-stack:id=stack-id;after=["other-stack"]
//
// For the operation "l" the [:param] is the link name, while <relative path>
// is the target of the symbolic link:
//
//	l:<target>:<link name>
//
// So this:
//
//	l:dir/file:dir/link
//
// Is equivalent to:
//
//	ln -s dir/file dir/link
//
// This is an internal mini-lang used to simplify testcases, so it expects well
// formed layout specification.
func (s S) BuildTree(layout []string) {
	s.t.Helper()

	buildTree(s.t, s.Config(), layout)
}

// IsGit tells if the sandbox is a git repository.
func (s S) IsGit() bool { return s.git != nil }

// Git returns a git wrapper that is useful to run git commands safely inside
// the test env repo.
func (s S) Git() *Git {
	if s.git == nil {
		s.t.Fatal("git not initialized for the sandbox")
	}

	return s.git
}

// Generate generates code for all stacks on the sandbox
func (s S) Generate() generate.Report {
	return s.GenerateWith(s.Config(), project.NewPath("/modules"))
}

// GenerateWith generates code for all stacks inside the provided path.
func (s S) GenerateWith(root *config.Root, vendorDir project.Path) generate.Report {
	t := s.t
	t.Helper()

	report := generate.Do(root, vendorDir, nil)
	for _, failure := range report.Failures {
		t.Errorf("Generate unexpected failure: %v", failure)
	}
	return report
}

// LoadStack load the stack given its relative path.
func (s S) LoadStack(dir project.Path) *config.Stack {
	s.t.Helper()

	st, err := config.LoadStack(s.Config(), dir)
	assert.NoError(s.t, err)

	return st
}

// LoadStacks load all stacks from sandbox rootdir.
func (s S) LoadStacks() config.List[*config.SortableStack] {
	s.t.Helper()

	entries, err := stack.List(s.Config().Tree())
	assert.NoError(s.t, err)

	var stacks config.List[*config.SortableStack]
	for _, entry := range entries {
		stacks = append(stacks, entry.Stack.Sortable())
	}

	return stacks
}

// LoadStackGlobals loads globals for specific stack on the sandbox.
// Fails the caller test if an error is found.
func (s S) LoadStackGlobals(
	root *config.Root,
	st *config.Stack,
) *eval.Object {
	s.t.Helper()

	report := globals.ForStack(root, st)
	assert.NoError(s.t, report.AsError())
	return report.Globals
}

// RootDir returns the root directory of the test env. All dirs/files created
// through the test env will be included inside this dir.
//
// It is a programming error to delete this dir, it will be automatically
// removed when the test finishes.
func (s S) RootDir() string {
	return s.rootdir
}

// Config returns the root configuration for the sandbox.
// It memoizes the parsing output, then multiple calls will parse
// the configuration once.
func (s *S) Config() *config.Root {
	s.t.Helper()
	if s.cfg != nil {
		return s.cfg
	}
	cfg, err := config.LoadRoot(s.RootDir())
	assert.NoError(s.t, err)
	s.cfg = cfg
	return cfg
}

// ReloadConfig reloads the sandbox configuration.
func (s *S) ReloadConfig() *config.Root {
	s.t.Helper()
	s.cfg = nil
	return s.Config()
}

// RootEntry returns a DirEntry for the root directory of the test env.
func (s S) RootEntry() DirEntry {
	return s.DirEntry(".")
}

// CreateModule will create a module dir with the given relative path, returning
// a directory entry that can be used to create files inside the module dir.
func (s S) CreateModule(relpath string) DirEntry {
	t := s.t
	t.Helper()

	if filepath.IsAbs(relpath) {
		t.Fatalf("CreateModule() needs a relative path but given %q", relpath)
	}

	return newDirEntry(s.t, s.rootdir, relpath)
}

// CreateStack will create a stack dir with the given relative path and
// initializes the stack, returning a stack entry that can be used
// to create files inside the stack dir.
//
// If the path is absolute, it will be considered in relation to the sandbox
// root dir.
func (s S) CreateStack(relpath string) StackEntry {
	t := s.t
	t.Helper()

	if filepath.IsAbs(relpath) {
		relpath = relpath[1:]
	}

	st := newStackEntry(t, s.RootDir(), relpath)
	assert.NoError(t, stack.Create(
		s.Config(),
		config.Stack{Dir: project.PrjAbsPath(s.RootDir(), st.Path())},
	))
	return st
}

// StackEntry gets the stack entry of the stack identified by relpath.
// The stack must exist (previously created).
func (s S) StackEntry(relpath string) StackEntry {
	return newStackEntry(s.t, s.RootDir(), relpath)
}

// DirEntry gets the dir entry for relpath.
// The dir must exist and must be a relative path to the sandbox root dir.
// The relpath must be a forward-slashed path.
func (s S) DirEntry(relpath string) DirEntry {
	t := s.t
	t.Helper()

	if strings.Contains(relpath, `\`) {
		panic("relpath requires a forward-slashed path")
	}

	if path.IsAbs(relpath) {
		t.Fatalf("DirEntry() needs a relative path but given %q", relpath)
	}

	abspath := filepath.Join(s.rootdir, filepath.FromSlash(relpath))
	stat, err := os.Stat(abspath)
	if err != nil {
		t.Fatalf("DirEntry(): dir must exist: %v", err)
	}

	if !stat.IsDir() {
		t.Fatalf("DirEntry(): %q is not directory", abspath)
	}

	return DirEntry{
		t:       t,
		abspath: abspath,
		relpath: relpath,
	}
}

// CreateFile will create a file inside this dir entry with the given name and
// the given body. The body can be plain text or a format string identical to
// what is defined on Go fmt package.
//
// It returns a file entry that can be used to further manipulate the created
// file, like replacing its contents. The file entry is optimized for always
// replacing the file contents, not streaming (using file as io.Writer).
//
// If the file already exists its contents will be truncated, like os.Create
// behavior: https://pkg.go.dev/os#Create
func (de DirEntry) CreateFile(name, body string, args ...interface{}) FileEntry {
	de.t.Helper()

	fe := FileEntry{
		t:        de.t,
		rootpath: de.rootpath,
		hostpath: filepath.Join(de.abspath, name),
	}
	fe.Write(body, args...)

	return fe
}

// ListGenFiles lists all files generated by Terramate for this dir entry.
func (de DirEntry) ListGenFiles(root *config.Root) []string {
	de.t.Helper()

	files, err := generate.ListGenFiles(root, de.abspath)
	assert.NoError(de.t, err, "listing dir generated files")
	return files
}

// CreateConfig will create a Terramate config file inside this dir entry with
// the given body using the default Terramate config filename.
//
// It returns a file entry that can be used to further manipulate the created
// file, like replacing its contents. The file entry is optimized for always
// replacing the file contents, not streaming (using file as io.Writer).
//
// If the file already exists its contents will be truncated, like os.Create
// behavior: https://pkg.go.dev/os#Create
func (de DirEntry) CreateConfig(body string) FileEntry {
	de.t.Helper()

	fe := FileEntry{
		t:        de.t,
		hostpath: filepath.Join(de.abspath, config.DefaultFilename),
	}
	fe.Write(body)
	return fe
}

// DeleteConfig deletes the default Terramate config file.
func (de DirEntry) DeleteConfig() {
	de.t.Helper()

	assert.NoError(de.t,
		os.Remove(filepath.Join(de.abspath, config.DefaultFilename)),
		"removing default configuration file")
}

// CreateDir creates a directory inside the dir entry directory. The relpath
// must be relative to the stack directory.
func (de DirEntry) CreateDir(relpath string) DirEntry {
	return newDirEntry(de.t, de.abspath, relpath)
}

// Chmod does the same as [test.Chmod] for the given file/dir inside
// this DirEntry.
func (de DirEntry) Chmod(relpath string, mode fs.FileMode) {
	test.Chmod(de.t, filepath.Join(de.abspath, relpath), mode)
}

// ReadFile will read a file inside this dir entry with the given name.
// It will fail the test if the file doesn't exist, since it assumes an
// expectation on the file being there.
func (de DirEntry) ReadFile(name string) []byte {
	de.t.Helper()
	return test.ReadFile(de.t, de.abspath, name)
}

// RemoveFile will delete a file inside this dir entry with the given name.
// It will succeeds if the file already doesn't exist.
func (de DirEntry) RemoveFile(name string) {
	de.t.Helper()
	test.RemoveFile(de.t, de.abspath, name)
}

// Path returns the absolute path of the directory entry.
func (de DirEntry) Path() string {
	return de.abspath
}

// RelPath returns the relative path of the directory entry
// using the host path separator.
func (de DirEntry) RelPath() string {
	return filepath.FromSlash(de.relpath)
}

// Git returns a Git wrapper for the dir.
func (de DirEntry) Git() *Git {
	git := NewGit(de.t, de.abspath)
	return git
}

// Write writes the given text body on the file, replacing its contents.
// The body can be plain text or a format string identical to what is defined on
// Go fmt package.
//
// It behaves like os.WriteFile: https://pkg.go.dev/os#WriteFile
func (fe FileEntry) Write(body string, args ...interface{}) {
	fe.t.Helper()

	body = fmt.Sprintf(body, args...)

	if err := os.WriteFile(fe.hostpath, []byte(body), 0700); err != nil {
		fe.t.Fatalf("os.WriteFile(%q) = %v", fe.hostpath, err)
	}
}

// Chmod changes the file mod, like os.Chmod.
func (fe FileEntry) Chmod(mode os.FileMode) {
	fe.t.Helper()

	test.Chmod(fe.t, fe.hostpath, mode)
}

// HostPath returns the absolute path of the file.
func (fe FileEntry) HostPath() string {
	return fe.hostpath
}

// Path returns the absolute project path of the file.
func (fe FileEntry) Path() string {
	return project.PrjAbsPath(fe.rootpath, fe.hostpath).String()
}

// ModSource returns the relative import path for the module with the given
// module dir entry. The path is relative to stack dir itself (hence suitable to
// be a module source path).
func (se StackEntry) ModSource(mod DirEntry) string {
	relpath, err := filepath.Rel(se.abspath, mod.abspath)
	assert.NoError(se.t, err)
	return filepath.ToSlash(relpath)
}

// WriteConfig will create a terramate configuration file on the stack
// or replace the current config if there is one.
func (se StackEntry) WriteConfig(cfg hcl.Config) {
	var out bytes.Buffer
	assert.NoError(se.t, hcl.PrintConfig(&out, cfg))
	se.DirEntry.CreateFile(config.DefaultFilename, out.String())
}

// DeleteStackConfig deletes the default stack definition file.
func (se StackEntry) DeleteStackConfig() {
	se.t.Helper()

	test.RemoveFile(se.t, se.abspath, stack.DefaultFilename)
}

// ReadFile will read the given file that must be located inside the stack.
func (se StackEntry) ReadFile(filename string) string {
	se.t.Helper()
	return string(se.DirEntry.ReadFile(filename))
}

// Load loads the terramate stack instance for this stack dir entry.
func (se StackEntry) Load(root *config.Root) *config.Stack {
	se.t.Helper()
	loadedStack, err := config.LoadStack(root, project.PrjAbsPath(root.HostDir(), se.Path()))
	assert.NoError(se.t, err)
	return loadedStack
}

// Path returns the absolute path of the stack.
func (se StackEntry) Path() string {
	return se.DirEntry.abspath
}

// RelPath returns the relative path of the stack. It is relative to the base
// dir of the test environment that created this stack.
func (se StackEntry) RelPath() string {
	return se.DirEntry.relpath
}

func newDirEntry(t testing.TB, rootdir string, relpath string) DirEntry {
	t.Helper()

	abspath := filepath.Join(rootdir, relpath)
	test.MkdirAll(t, abspath)

	return DirEntry{
		t:        t,
		rootpath: rootdir,
		abspath:  abspath,
		relpath:  relpath,
	}
}

func newStackEntry(t testing.TB, rootdir string, relpath string) StackEntry {
	return StackEntry{DirEntry: newDirEntry(t, rootdir, relpath)}
}

func parseListSpec(t testing.TB, name, value string) []string {
	if !strings.HasPrefix(value, "[") ||
		!strings.HasSuffix(value, "]") {
		t.Fatalf("malformed %q value: %q", name, value)
	}
	quotedList := strings.Split(value[1:len(value)-1], ",")
	list := make([]string, 0, len(quotedList))
	for _, l := range quotedList {
		l = strings.TrimSpace(l)
		if l == "" {
			continue
		}

		if !strings.HasPrefix(l, `"`) {
			t.Fatalf("expect quoted strings but given %q", l)
		}

		var err error
		val, err := strconv.Unquote(l)
		assert.NoError(t, err, "list item not properly quoted")

		list = append(list, val)
	}

	return list
}

func buildTree(t testing.TB, root *config.Root, layout []string) {
	t.Helper()

	rootdir := root.HostDir()
	parsePathData := func(spec string) (string, string) {
		tmp := spec[2:]
		if len(tmp) == 0 {
			// relative to s.rootdir
			return ".", ""
		}
		index := strings.IndexByte(tmp, ':')
		if index == -1 {
			return tmp, ""
		}
		path := tmp[0:index]
		data := tmp[index+1:]
		return path, data
	}

	gentmfile := func(relpath, data string) {
		attrs := strings.Split(data, ";")

		cfgdir := filepath.Join(rootdir, filepath.FromSlash(relpath))
		test.MkdirAll(t, cfgdir)
		cfg, err := hcl.NewConfig(cfgdir)
		assert.NoError(t, err)

		cfg.Stack = &hcl.Stack{}

		for _, attr := range attrs {
			parts := strings.Split(attr, "=")
			name := parts[0]
			value := parts[1]
			switch name {
			case "id":
				cfg.Stack.ID = value
			case "after":
				cfg.Stack.After = parseListSpec(t, name, value)
			case "before":
				cfg.Stack.Before = parseListSpec(t, name, value)
			case "wants":
				cfg.Stack.Wants = parseListSpec(t, name, value)
			case "wanted_by":
				cfg.Stack.WantedBy = parseListSpec(t, name, value)
			case "watch":
				cfg.Stack.Watch = parseListSpec(t, name, value)
			case "description":
				cfg.Stack.Description = value
			case "tags":
				cfg.Stack.Tags = parseListSpec(t, name, value)
			default:
				t.Fatalf("attribute " + parts[0] + " not supported.")
			}
		}

		assert.NoError(t, cfg.Save(config.DefaultFilename),
			"BuildTree() failed to generate config file.")
	}

	for _, spec := range layout {
		path, data := parsePathData(spec)

		specKind := string(spec[0:2])
		switch specKind {
		case "d:":
			test.MkdirAll(t, filepath.Join(rootdir, spec[2:]))
		case "l:":
			target := filepath.Join(rootdir, path)
			linkName := filepath.Join(rootdir, data)
			test.Symlink(t, target, linkName)
		case "g:":
			repodir := filepath.Join(rootdir, spec[2:])
			test.MkdirAll(t, repodir)
			git := NewGit(t, repodir)
			git.Init()
		case "s:":
			if data == "" {
				abspath := filepath.Join(rootdir, path)
				stackdir := project.PrjAbsPath(rootdir, abspath)
				assert.NoError(t, stack.Create(root, config.Stack{Dir: stackdir}))
				continue
			}

			gentmfile(path, data)
		case "f:":
			test.WriteFile(t, rootdir, path, data)
		default:
			t.Fatalf("unknown spec kind: %q", specKind)
		}
	}
}
