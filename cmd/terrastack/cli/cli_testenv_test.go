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

// TestEnv is a full test env with its own base dir that is an initialized
// git repo for test purposes.
type TestEnv struct {
	t       *testing.T
	git     *Git
	basedir string
}

// Git is a git wrapper that makes testing easy by handling
// errors automatically, failing the caller test.
type Git struct {
	t       *testing.T
	g       *git.Git
	basedir string
}

// DirEntry represents a directory and can be
// used to create files inside the directory
type DirEntry struct {
	t       *testing.T
	pathabs string
	pathrel string
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
//
// It is also a programming error to use TestEnv on Parallel
// tests since the test env will change global things like
// env vars and the PWD.
func NewTestEnv(t *testing.T) *TestEnv {
	t.Helper()

	basedir := t.TempDir()

	git := &Git{
		t:       t,
		g:       test.NewGitWrapper(t, basedir, false),
		basedir: basedir,
	}

	git.Init()
	return &TestEnv{
		t:       t,
		git:     git,
		basedir: basedir,
	}
}

// Git returns a git wrapper that is useful to run git commands
// safely inside the test env repo.
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

// Debug will creative an interactive shell inside the test environment
// basedir so you can inspect it. It will interrupt your test execution.
func (te *TestEnv) Debug() {
	te.t.Helper()

	cmd := exec.Command("bash")
	cmd.Dir = te.basedir
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		te.t.Errorf("TestEnv.Debug(); error=%q", err)
	}
}

// CreateModule will create a module dir with the given name
// returning a directory entry that can be used to
// create files inside the module dir.
//
// It is a programming error to call this method with a module
// name that already exists.
func (te *TestEnv) CreateModule(name string) DirEntry {
	te.t.Helper()

	return newDirEntry(te.t, te.basedir, filepath.Join("modules", name))
}

// CreateStack will create a stack dir with the given name
// returning a stack entry that can be used to
// create files inside the stack dir.
//
// It is a programming error to call this method with a stack
// name that already exists.
func (te *TestEnv) CreateStack(name string) *StackEntry {
	te.t.Helper()

	// Given the current design assuming ../../modules is safe
	// But we could change this in the future and maintain the
	// current API working.
	return &StackEntry{
		DirEntry:       newDirEntry(te.t, te.basedir, filepath.Join("stacks", name)),
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
func (de DirEntry) CreateFile(name, body string, args ...interface{}) *FileEntry {
	de.t.Helper()

	fe := &FileEntry{
		t:    de.t,
		path: filepath.Join(de.pathabs, name),
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

// Path returns the absolute path of the file.
func (fe *FileEntry) Path() string {
	return fe.path
}

// ModImportPath returns the relative import path for the
// module with the given name. The path is relative to
// stack dir.
func (se *StackEntry) ModImportPath(name string) string {
	return filepath.Join(se.modulesRelPath, name)
}

// Path returns the absolute path of the stack.
func (se *StackEntry) Path() string {
	return se.DirEntry.pathabs
}

// PathRel returns the relative path of the stack.
// It is relative to the base dir of the test environment
// that created this stack.
func (se *StackEntry) PathRel() string {
	return se.DirEntry.pathrel
}

// Init will initialize the git repo
func (git *Git) Init() {
	git.t.Helper()

	if err := git.g.Init(git.basedir); err != nil {
		git.t.Errorf("Git.Init(%v) = %v", git.basedir, err)
	}
}

// Add will add files to the commit list
func (git *Git) Add(files ...string) {
	git.t.Helper()

	if err := git.g.Add(files...); err != nil {
		git.t.Errorf("Git.Add(%v) = %v", files, err)
	}
}

// Commit will commit previously added files
func (git *Git) Commit(msg string, args ...string) {
	git.t.Helper()

	if err := git.g.Commit(msg, args...); err != nil {
		git.t.Errorf("Git.Commit(%s, %v) = %v", msg, args, err)
	}
}

// Checkout will checkout a branch
func (git *Git) Checkout(rev string, create bool) {
	git.t.Helper()

	if err := git.g.Checkout(rev, create); err != nil {
		git.t.Errorf("Git.Checkout(%s, %v) = %v", rev, create, err)
	}
}

func newDirEntry(t *testing.T, basedir string, pathrel string) DirEntry {
	t.Helper()

	pathabs := filepath.Join(basedir, pathrel)
	test.MkdirAll(t, pathabs)

	return DirEntry{
		t:       t,
		pathabs: pathabs,
		pathrel: pathrel,
	}
}
