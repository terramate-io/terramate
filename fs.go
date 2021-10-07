package terrastack

import (
	"fmt"
	"io"
	"io/fs"
	"os"
)

type (
	// FS is the terrastack writable filesystem interface.
	FS interface {
		fs.FS
		Stat(name string) (fs.FileInfo, error)
		Create(name string) (fs.File, error)
		Remove(name string) error
	}

	osFS struct{}

	// FSPath is a generic filesystem path.
	FSPath string

	// Dirname is a filesystem directory path.
	Dirname string

	// Filename is a filesystem file path.
	Filename string

	// Path represents any kind of path.
	Path interface {
		Check() (exists bool, err error)
	}
)

// Check if p is an existent filesystem path.
func (p FSPath) Check() (exists bool, err error) {
	_, err = afs.Stat(string(p))
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}

		return false, fmt.Errorf("stat failed: %w", err)
	}

	return true, nil
}

// Check if d is an existing directory.
func (d Dirname) Check() (exists bool, err error) {
	st, err := afs.Stat(string(d))
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}

		return false, fmt.Errorf("stat failed: %w", err)
	}

	if !st.IsDir() {
		return false, fmt.Errorf("is not a directory")
	}

	return true, nil
}

// Check if f is an existing regular file.
func (f Filename) Check() (exists bool, err error) {
	st, err := afs.Stat(string(f))
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}

		return false, fmt.Errorf("stat failed: %w", err)
	}

	if !st.Mode().IsRegular() {
		return false, fmt.Errorf("is not a regular file")
	}

	return true, nil
}

func (afs osFS) Open(name string) (fs.File, error) {
	return os.Open(name)
}

func (afs osFS) Stat(name string) (fs.FileInfo, error) {
	return os.Stat(name)
}

func (afs osFS) Create(name string) (fs.File, error) {
	return os.Create(name)
}

func (afs osFS) Remove(name string) error { return os.Remove(name) }

// WriteFile writes the content of data in the filename name.
func WriteFile(afs FS, name Filename, data []byte) error {
	file, err := afs.Create(string(name))
	if err != nil {
		return fmt.Errorf("writefile: %w", err)
	}

	defer file.Close()

	fwriter, ok := file.(io.WriteCloser)
	if !ok {
		return fmt.Errorf("file is not writable")
	}

	total := 0

	for total < len(data) {
		n, err := fwriter.Write(data[total:])
		if err != nil {
			return err
		}

		total += n
	}

	return nil
}
