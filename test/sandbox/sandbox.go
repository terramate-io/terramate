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
	stdfs "io/fs"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/madlambda/spells/assert"
	"github.com/terramate-io/terramate/config"
	"github.com/terramate-io/terramate/fs"
	"github.com/terramate-io/terramate/generate"
	"github.com/terramate-io/terramate/globals"
	"github.com/terramate-io/terramate/hcl"
	"github.com/terramate-io/terramate/hcl/eval"
	"github.com/terramate-io/terramate/project"
	"github.com/terramate-io/terramate/run"
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

	Env []string
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
	s := NoGit(t, false)

	s.git = NewGit(t, s.RootDir())
	s.git.Init()
	s.commitGitIgnore()
	return s
}

// NewWithGitConfig creates a new sandbox using the cfg configuration for the
// git repository.
func NewWithGitConfig(t testing.TB, cfg GitConfig) S {
	s := NoGit(t, false)

	cfg.repoDir = s.RootDir()

	s.git = NewGitWithConfig(t, cfg)
	s.git.Init()
	s.commitGitIgnore()
	return s
}

// NoGit creates a new test sandbox with no git repository.
//
// It is a programming error to use a test env created with a testing.TB other
// than the one of the test using the test env, for a new test/sub-test always
// create a new test env for it.
func NoGit(t testing.TB, createProject bool) S {
	t.Helper()

	// here we create some stacks outside the root directory of the
	// sandbox so we can check if terramate does not ascend to parent
	// directories.

	outerDir := test.TempDir(t)
	buildTree(t, config.NewRoot(config.NewTree(outerDir)), nil, []string{
		"s:this-stack-must-never-be-visible",
		"s:other-hidden-stack",
	})

	rootdir := filepath.Join(outerDir, "sandbox")
	test.MkdirAll(t, rootdir)

	if createProject {
		test.WriteRootConfig(t, rootdir)
	}

	return S{
		t:       t,
		rootdir: test.CanonPath(t, rootdir),
	}
}

func (s S) commitGitIgnore() {
	s.BuildTree([]string{"file:.gitignore:" + defaultGitIgnoreContent})
	s.git.CommitAll("add gitignore")
}

// BuildTree builds a tree layout based on the layout specification.
// Each string in the slice represents a filesystem operation, and each
// operation has the format below:
//
//	<kind>:<param1>[:param2]
//
// Where kind is one of the below:
//
//	"d" or "dir" for directory creation.
//	"g" or "git" for local git directory creation.
//	"s" or "stack" for initialized stacks.
//	"f" or "file" for file creation.
//	"l" or "link" for symbolic link creation.
//	"copy" for copying directories or files.
//	"run" for executing a command inside directory.
//
// And [:param2] is optional and it depends on the kind.
//
// For the operations "f" and "s" [:param2] is defined as:
//
//	For "f" it is the content of the file to be created.
//	For "s" it is a key value pair of the form:
//	  <attr1>=<val1>[;<attr2>=<val2>]
//
// Where attrN is a string attribute of the terramate block of the stack.
// Example:
//
//	s:name-of-the-stack:id=stack-id;after=["other-stack"]
//
// For the operation "l" the [:param2] is the link name, while <param1>
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
// For "copy" the [:param1] is the source directory and [:param2] is the target
// directory.
//
// For "run" the [:param1] is the working directory and [:param2] is the command
// to be executed.
//
// This is an internal mini-lang used to simplify testcases, so it expects well
// formed layout specification.
func (s S) BuildTree(layout []string) {
	s.t.Helper()

	buildTree(s.t, s.Config(), s.Env, layout)
}

// AssertTree compares the current tree against the given layout specification.
// The specification works similar to BuildTree.
//
// The following directives are supported:
//
// "s:<stackpath>" or "stack:..."
// Assert that stackpath is a stack.
//
// "d:<dirpath>" or "dir:..."
// Assert that dirpath is an existing directory.
//
// "f:<filepath>[:<content>]" or "file:..."
// Assert that filepath is an existing file, optionally having the given content.
func (s S) AssertTree(layout []string, opts ...AssertTreeOption) {
	s.t.Helper()

	o := &assertTreeOptions{}
	for _, opt := range opts {
		opt(o)
	}

	assertTree(s.t, s.Config(), layout, o)
}

// AssertTreeOption is the common option type for AssertTree.
type AssertTreeOption func(o *assertTreeOptions)

// WithStrictStackValidation is an option for AssertTree to validate that
// the list of stacks given in the layout _exactly_ matches the list of stacks
// in the tree, i.e. it doesn't just check that the given stacks exist, but also
// that there are no other stacks beyond that.
func WithStrictStackValidation() AssertTreeOption {
	return func(o *assertTreeOptions) { o.withStrictStacks = true }
}

type assertTreeOptions struct {
	withStrictStacks bool
	//TODO(snk): add withStrictFiles
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
func (de DirEntry) Chmod(relpath string, mode stdfs.FileMode) {
	test.AssertChmod(de.t, filepath.Join(de.abspath, relpath), mode)
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

	test.MkdirAll(fe.t, filepath.Dir(fe.hostpath))

	if err := os.WriteFile(fe.hostpath, []byte(body), 0700); err != nil {
		fe.t.Fatalf("os.WriteFile(%q) = %v", fe.hostpath, err)
	}
}

// Chmod changes the file mod, like os.Chmod.
func (fe FileEntry) Chmod(mode os.FileMode) {
	fe.t.Helper()

	test.AssertChmod(fe.t, fe.hostpath, mode)
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

func buildTree(t testing.TB, root *config.Root, environ []string, layout []string) {
	t.Helper()

	rootdir := root.HostDir()
	parseParams := func(spec string) (string, string) {
		colonIndex := strings.Index(spec, ":") + 1
		tmp := spec[colonIndex:]
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

	genStackFile := func(relpath, data string) {
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
		param1, param2 := parseParams(spec)
		colonIndex := strings.Index(spec, ":") + 1
		specKind := string(spec[0:colonIndex])
		switch specKind {
		case "d:", "dir:":
			test.MkdirAll(t, filepath.Join(rootdir, spec[colonIndex:]))
		case "l:", "link:":
			target := filepath.Join(rootdir, param1)
			linkName := filepath.Join(rootdir, param2)
			test.Symlink(t, target, linkName)
		case "g:", "git:":
			repodir := filepath.Join(rootdir, spec[colonIndex:])
			test.MkdirAll(t, repodir)
			git := NewGit(t, repodir)
			git.Init()
		case "s:", "stack:":
			if param2 == "" {
				abspath := filepath.Join(rootdir, param1)
				stackdir := project.PrjAbsPath(rootdir, abspath)
				assert.NoError(t, stack.Create(root, config.Stack{Dir: stackdir}))
				continue
			}

			genStackFile(param1, param2)
		case "f:", "file:":
			test.WriteFile(t, rootdir, param1, param2)
		case "copy:":
			assert.NoError(t, fs.CopyAll(
				filepath.Join(rootdir, param1),
				param2,
			))
		case "run:":
			cmdParts := strings.Split(param2, " ")
			path, err := run.LookPath(cmdParts[0], environ)
			assert.NoError(t, err)
			cmd := exec.Command(path, cmdParts[1:]...)
			cmd.Dir = filepath.Join(rootdir, param1)
			cmd.Env = environ
			out, err := cmd.CombinedOutput()
			assert.NoError(t, err, "failed to execute sandbox run: (output: %s)", out)
		default:
			t.Fatalf("unknown spec kind: %q", specKind)
		}
	}
}

func assertTree(t testing.TB, root *config.Root, layout []string, opts *assertTreeOptions) {
	t.Helper()

	popArg := func(spec string) (string, string) {
		idx := strings.Index(spec, ":")
		if idx == -1 {
			return spec, ""
		}
		return spec[0:idx], spec[idx+1:]
	}

	rootdir := root.HostDir()

	wantStrictStacks := []string{}

	for _, spec := range layout {
		specKind, spec := popArg(spec)

		switch specKind {
		case "d", "dir":
			dirname, _ := popArg(spec)
			test.IsDir(t, rootdir, dirname)

		case "f", "file":
			fname, spec := popArg(spec)
			want, _ := popArg(spec)

			if want != "" {
				got := string(test.ReadFile(t, rootdir, fname))
				assert.EqualStrings(t, want, got, "want:\n%s\ngot:\n%s\n", want, got)
			} else {
				test.IsFile(t, rootdir, fname)
			}

		case "!e", "not_exist":
			fname, _ := popArg(spec)
			test.DoesNotExist(t, rootdir, fname)

		case "s", "stack":
			stackdir, _ := popArg(spec)
			prjStackdir := project.PrjAbsPath(rootdir, stackdir)

			if opts.withStrictStacks {
				wantStrictStacks = append(wantStrictStacks, prjStackdir.String())
			} else {
				_, err := config.LoadStack(root, prjStackdir)
				assert.NoError(t, err, "not a valid stack")
			}

		default:
			t.Fatalf("unknown spec kind: %q", specKind)
		}
	}

	if opts.withStrictStacks {
		gotStrictStacks := []string{}
		stackEntries, err := stack.List(root.Tree())
		assert.NoError(t, err)

		for _, st := range stackEntries {
			gotStrictStacks = append(gotStrictStacks, st.Stack.Dir.String())
		}

		sort.Strings(wantStrictStacks)
		sort.Strings(gotStrictStacks)

		if diff := cmp.Diff(wantStrictStacks, gotStrictStacks); diff != "" {
			t.Errorf("stack list mismatch (-want +got): %s", diff)
		}
	}
}

const defaultGitIgnoreContent = `
# Local .terraform directories
**/.terraform/*

# .tfstate files
*.tfstate
*.tfstate.*

# Crash log files
crash.log
crash.*.log

# Exclude all .tfvars files, which are likely to contain sensitive data, such as
# password, private keys, and other secrets. These should not be part of version 
# control as they are data points which are potentially sensitive and subject 
# to change depending on the environment.
*.tfvars
*.tfvars.json

# Ignore override files as they are usually used to override resources locally and so
# are not checked in
override.tf
override.tf.json
*_override.tf
*_override.tf.json

# Include override files you do wish to add to version control using negated pattern
# !example_override.tf

# Include tfplan files to ignore the plan output of command: terraform plan -out=tfplan
# example: *tfplan*

# Ignore CLI configuration files
.terraformrc
terraform.rc
`
