// Package sandbox provides an easy way to setup
// isolated terrastack projects that can be used
// on testing, acting like sandboxes.
//
// It helps with:
//
// - git initialization/operations
// - Terraform module creation
// - Terrastack stack creation
package sandbox

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/mineiros-io/terrastack/git"
	"github.com/mineiros-io/terrastack/test"
)

// S is a full sandbox with its own base dir that is an initialized
// git repo for test purposes.
type S struct {
	t       *testing.T
	git     Git
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
	abspath string
	relpath string
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

// New creates a new test sandbox.
//
// It is a programming error to use a test env created
// with a *testing.T other than the one of the test
// using the test env, for a new test/sub-test always create
// a new test env for it.
func New(t *testing.T) S {
	t.Helper()

	basedir := t.TempDir()

	git := Git{
		t:       t,
		g:       test.NewGitWrapper(t, basedir, false),
		basedir: basedir,
	}
	git.Init()
	return S{
		t:       t,
		git:     git,
		basedir: basedir,
	}
}

// Git returns a git wrapper that is useful to run git commands
// safely inside the test env repo.
func (s S) Git() Git {
	return s.git
}

// BaseDir returns the base dir of the test env.
// All dirs/files created through the test env will
// be included inside this dir.
//
// It is a programming error to delete this dir, it will
// be automatically removed when the test finishes.
func (s S) BaseDir() string {
	return s.basedir
}

// CreateModule will create a module dir with the given name
// returning a directory entry that can be used to
// create files inside the module dir.
//
// It is a programming error to call this method with a module
// name that already exists on this test env.
func (s S) CreateModule(name string) DirEntry {
	s.t.Helper()

	return newDirEntry(s.t, s.basedir, filepath.Join("modules", name))
}

// CreateStack will create a stack dir with the given name
// returning a stack entry that can be used to
// create files inside the stack dir.
//
// It is a programming error to call this method with a stack
// name that already exists on this test env.
func (s S) CreateStack(name string) *StackEntry {
	s.t.Helper()

	// Given the current design assuming ../../modules is safe
	// But we could change this in the future and maintain the
	// current API working.
	return &StackEntry{
		DirEntry:       newDirEntry(s.t, s.basedir, filepath.Join("stacks", name)),
		modulesRelPath: "../../modules",
	}
}

// CreateFile will create a file inside this dir entry with the
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
		path: filepath.Join(de.abspath, name),
	}
	fe.Write(body, args...)

	return fe
}

// Write writes the given text body on the file, replacing its contents.
// The body can be plain text or a format string identical to what
// is defined on Go fmt package.
//
// It behaves like os.WriteFile: https://pkg.go.dev/os#WriteFile
func (fe FileEntry) Write(body string, args ...interface{}) {
	fe.t.Helper()

	body = fmt.Sprintf(body, args...)

	if err := os.WriteFile(fe.path, []byte(body), 0700); err != nil {
		fe.t.Fatalf("os.WriteFile(%q) = %v", fe.path, err)
	}
}

// Path returns the absolute path of the file.
func (fe FileEntry) Path() string {
	return fe.path
}

// ModSource returns the relative import path for the
// module with the given name. The path is relative to
// stack dir itself (hence suitable to be an source path).
func (se StackEntry) ModSource(name string) string {
	return filepath.Join(se.modulesRelPath, name)
}

// Path returns the absolute path of the stack.
func (se StackEntry) Path() string {
	return se.DirEntry.abspath
}

// RelPath returns the relative path of the stack.
// It is relative to the base dir of the test environment
// that created this stack.
func (se StackEntry) RelPath() string {
	return se.DirEntry.relpath
}

// Init will initialize the git repo
func (git Git) Init() {
	git.t.Helper()

	if err := git.g.Init(git.basedir); err != nil {
		git.t.Fatalf("Git.Init(%v) = %v", git.basedir, err)
	}
}

// Add will add files to the commit list
func (git Git) Add(files ...string) {
	git.t.Helper()

	if err := git.g.Add(files...); err != nil {
		git.t.Fatalf("Git.Add(%v) = %v", files, err)
	}
}

// Commit will commit previously added files
func (git Git) Commit(msg string, args ...string) {
	git.t.Helper()

	if err := git.g.Commit(msg, args...); err != nil {
		git.t.Fatalf("Git.Commit(%s, %v) = %v", msg, args, err)
	}
}

// Checkout will checkout a branch
func (git Git) Checkout(rev string, create bool) {
	git.t.Helper()

	if err := git.g.Checkout(rev, create); err != nil {
		git.t.Fatalf("Git.Checkout(%s, %v) = %v", rev, create, err)
	}
}

func newDirEntry(t *testing.T, basedir string, relpath string) DirEntry {
	t.Helper()

	abspath := filepath.Join(basedir, relpath)
	test.MkdirAll(t, abspath)

	return DirEntry{
		t:       t,
		abspath: abspath,
		relpath: relpath,
	}
}
