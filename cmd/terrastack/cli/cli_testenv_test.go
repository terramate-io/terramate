package cli_test

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/mineiros-io/terrastack/git"
	"github.com/mineiros-io/terrastack/test"
)

type TestEnv struct {
	t       *testing.T
	git     *Git
	basedir string
}

type Git struct {
	t       *testing.T
	g       *git.Git
	basedir string
}

// DirEntry represents a directory and can be
// used to create files inside the directory
type DirEntry struct {
	t    *testing.T
	path string
}

// StackEntry represents a directory that has a stack
// inside, it extends a DirEntry with stack specific
// functionality.
type StackEntry struct {
	DirEntry
	modulesRelPath string
}

// FileEntry represents a file and can be used
// to manipulate the file contents.
// It is optimized for reading/writing all contents,
// not stream programming (io.Reader/io.Writer).
// It has limited usefulness but it is easier to
// work with for testing.
type FileEntry struct {
	t    *testing.T
	path string
}

// NewTestEnv creates a new test env, including a new
// temporary repository. All commands run using the test
// env will use this tmp dir as the working dir.
//
// It is a programming error to use a test env created
// with a *testing.T other than the one of the test
// using the test env, for a new test/sub-test always create
// a new test env for it.
func NewTestEnv(t *testing.T) *TestEnv {
	t.Helper()

	basedir := test.TempDir(t, "")
	git := &Git{
		t:       t,
		g:       test.NewGitWrapper(t, basedir, false),
		basedir: basedir,
	}

	git.Init()
	return &TestEnv{t: t, git: git, basedir: basedir}
}

// Cleanup will release any resources, like files, created
// by the test env, it is a programming error to use the test
// env (or any object created from it) after calling this method.
func (te *TestEnv) Cleanup() {
	te.t.Helper()

	test.RemoveAll(te.t, te.basedir)
}

// Git returns a git wrapper that is useful to run git commands
// safely inside the test env dir.
func (te *TestEnv) Git() *Git {
	return te.git
}

// BaseDir returns the base dir of the test env.
func (te *TestEnv) BaseDir() string {
	return te.basedir
}

// Run will run the given cmd with the provided args using
// the test env base dir as the command working dir.
// This method fails the test if the command fails, where
// a command failed is defined by its status code (!= 0).
func (te *TestEnv) Run(name string, args ...string) {
	te.t.Helper()

	cmd := exec.Command(name, args...)
	cmd.Dir = te.basedir

	if out, err := cmd.CombinedOutput(); err != nil {
		te.t.Errorf("failed to run: '%v' err: '%v' output: '%s'", cmd, err, string(out))
	}
}

// CreateModule will create a module dir with the given name
// returning a directory entry that can be used to
// create files inside the module dir.
//
// It is a programming error to call this method with a module
// name that already exists.
func (te *TestEnv) CreateModule(name string) *DirEntry {
	te.t.Helper()

	moddir := filepath.Join(te.basedir, "modules", name)
	test.MkdirAll(te.t, moddir)

	return &DirEntry{t: te.t, path: moddir}
}

// CreateStack will create a stack dir with the given name
// returning a stack entry that can be used to
// create files inside the stack dir.
//
// It is a programming error to call this method with a stack
// name that already exists.
func (te *TestEnv) CreateStack(name string) *StackEntry {
	te.t.Helper()

	stackdir := filepath.Join(te.basedir, "stacks", name)
	test.MkdirAll(te.t, stackdir)

	// Given the current design assuming ../../modules is safe
	// But we could change this in the future and maintain the
	// current API working.
	return &StackEntry{
		DirEntry: DirEntry{
			t:    te.t,
			path: stackdir,
		},
		modulesRelPath: "../../modules",
	}
}

// CreateFile will create a file inside the dir with the
// given name and the given body. The body can be plain text
// or a format string identical to what is defined on Go fmt package.
//
// It returns a file entry that can be used to further
// manipulate the created file, like replacing its contents.
// The file entry is optimized for always replacing the
// file contents, not streaming (using file as io.Writer).
//
// If the file already exists its contents will be truncated,
// like os.Create behavior: https://pkg.go.dev/os#Create
func (de *DirEntry) CreateFile(name, body string, args ...interface{}) *FileEntry {
	de.t.Helper()

	fe := &FileEntry{
		t:    de.t,
		path: filepath.Join(de.path, name),
	}
	fe.Write(body, args...)

	return fe
}

// Write writes the given text body on the file, replacing its contents.
// The body can be plain text or a format string identical to what
// is defined on Go fmt package.
//
// It behaves like os.WriteFile: https://pkg.go.dev/os#WriteFile
func (fe *FileEntry) Write(body string, args ...interface{}) {
	fe.t.Helper()

	body = fmt.Sprintf(body, args...)

	if err := os.WriteFile(fe.path, []byte(body), 0700); err != nil {
		fe.t.Errorf("os.WriteFile(%q) = %v", fe.path, err)
	}
}

// ModImportPath returns the relative import path for the
// module with the given name. The path is relative to
// stack dir.
func (se *StackEntry) ModImportPath(name string) string {
	return filepath.Join(se.modulesRelPath, name)
}

// Path returns the relative path of the stack.
// It is relative to the base dir of the test environment
// that created this stack.
func (se *StackEntry) Path() string {
	return se.DirEntry.path
}

func (git *Git) Init() {
	git.t.Helper()

	if err := git.g.Init(git.basedir); err != nil {
		git.t.Errorf("Git.Init(%v) = %v", git.basedir, err)
	}
}

func (git *Git) Add(files ...string) {
	git.t.Helper()

	if err := git.g.Add(files...); err != nil {
		git.t.Errorf("Git.Add(%v) = %v", files, err)
	}
}

func (git *Git) Commit(msg string, args ...string) {
	git.t.Helper()

	if err := git.g.Commit(msg, args...); err != nil {
		git.t.Errorf("Git.Commit(%s, %v) = %v", msg, args, err)
	}
}
