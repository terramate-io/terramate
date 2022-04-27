package genfile

import (
	"github.com/mineiros-io/terramate"
	"github.com/mineiros-io/terramate/stack"
)

// StackFiles represents all generated files for a stack,
// mapping the generated file filename to the actual file body.
type StackFiles struct {
	files map[string]File
}

// File represents generated file from a single generate_file block.
type File struct {
	origin string
	body   string
}

// Body returns the file body.
func (f File) Body() string {
	return f.body
}

// Origin returns the path, relative to the project root,
// of the configuration that originated the file.
func (f File) Origin() string {
	return f.origin
}

// GeneratedFiles returns all generated files, mapping the name to
// the file description.
func (s StackFiles) GeneratedFiles() map[string]File {
	cp := map[string]File{}
	for k, v := range s.files {
		cp[k] = v
	}
	return cp
}

// Load loads and parse from the file system all generate_file blocks for
// a given stack. It will navigate the file system from the stack dir until
// it reaches rootdir, loading generate_file blocks found on Terramate
// configuration files.
//
// All generate_file blocks must have unique labels, even ones at different
// directories. Any conflicts will be reported as an error.
//
// Metadata and globals for the stack are used on the evaluation of the
// generate_file blocks.
//
// The rootdir MUST be an absolute path.
func Load(rootdir string, sm stack.Metadata, globals terramate.Globals) (StackFiles, error) {
	return StackFiles{}, nil
}
