package terrastack_test

import (
	"fmt"
	"io"
	"io/fs"
	"os"
	"time"

	"github.com/mineiros-io/terrastack"
)

type (
	// fakeFS implements a testing filesystem where the logic is guided by
	// the resource name.
	// It means opening a file named "/no/exists" should return an
	// os.ErrNotExists error.
	fakeFS struct {
		files map[string]*fakeFileData
	}

	fakeFileMode struct {
		name  string
		isdir bool
		size  int64
	}

	fakeFileData struct {
		name  string
		data  []byte
		isdir bool
	}

	fakeFile struct {
		data *fakeFileData
		pos  int
	}
)

func newFakeFS() *fakeFS {
	afs := &fakeFS{
		files: make(map[string]*fakeFileData),
	}

	newFakeFile(afs, "/stack/initialized/same/version", "", true)
	newFakeFile(afs, "/stack/initialized/same/version/terrastack",
		terrastack.Version(), false)

	newFakeFile(afs, "/stack/initialized/other/version", "", true)
	newFakeFile(afs, "/stack/initialized/other/version/terrastack",
		"9999.9999.9999", false)

	newFakeFile(afs, "/stack/not/initialized", "", true)

	return afs
}

func (fake fakeFS) Open(name string) (fs.File, error) {
	filedata, ok := fake.files[name]
	if !ok {
		return nil, os.ErrNotExist
	}

	return filedata.File(), nil
}

func (fake *fakeFS) Create(name string) (fs.File, error) {
	delete(fake.files, name)

	newFakeFile(fake, name, "", false)
	return fake.Open(name)
}

func (fake fakeFS) Remove(name string) error {
	if _, ok := fake.files[name]; !ok {
		return os.ErrNotExist
	}

	delete(fake.files, name)
	return nil
}

func (fake fakeFS) Stat(name string) (os.FileInfo, error) {
	switch name {
	case "/no/exists":
		return nil, os.ErrNotExist
	case "/no/permission":
		return nil, os.ErrPermission
	}

	filedata, ok := fake.files[name]
	if !ok {
		return nil, os.ErrNotExist
	}

	return filedata.File().Stat()
}

func (mode fakeFileMode) Name() string       { return mode.name }
func (mode fakeFileMode) IsDir() bool        { return mode.isdir }
func (mode fakeFileMode) ModTime() time.Time { return time.Now() }
func (mode fakeFileMode) Size() int64        { return mode.size }
func (mode fakeFileMode) Sys() interface{}   { return nil }
func (mode fakeFileMode) Mode() fs.FileMode {
	if mode.isdir {
		return os.ModeDir | 0644
	}
	return 0644
}

func (fd *fakeFileData) File() fs.File {
	return &fakeFile{
		data: fd,
	}
}

func newFakeFile(afs *fakeFS, name string, data string, isdir bool) {
	afs.files[name] = &fakeFileData{
		name:  name,
		data:  []byte(data),
		isdir: isdir,
	}
}

func (f *fakeFile) Read(data []byte) (int, error) {
	if f.data.isdir {
		return 0, fmt.Errorf("file is a directory")
	}

	if len(data) > 0 && f.pos >= len(f.data.data) {
		return 0, io.EOF
	}
	n := copy(data[:], f.data.data[f.pos:])
	f.pos += n
	return n, nil
}

func (f *fakeFile) Write(data []byte) (int, error) {
	if len(f.data.data[f.pos:]) < len(data) {
		newdata := make([]byte, len(f.data.data)+len(data)-len(f.data.data[f.pos:]))
		copy(newdata, f.data.data)
		f.data.data = newdata
	}

	n := copy(f.data.data[f.pos:], data)
	f.pos += n
	return n, nil
}

func (f fakeFile) Stat() (fs.FileInfo, error) {
	return fakeFileMode{
		name:  f.data.name,
		isdir: f.data.isdir,
		size:  int64(len(f.data.data)),
	}, nil
}

func (f fakeFile) Close() error { return nil }
