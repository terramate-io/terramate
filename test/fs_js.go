//go:build js

package test

import (
	"io/fs"
)

func chmod(fname string, mode fs.FileMode) error {
	panic("`chmod` is not implemented for wasm")
}
