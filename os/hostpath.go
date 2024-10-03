package os

import (
	"fmt"
	"path/filepath"
	"strings"
)

type Path string
type Paths []Path

func NewHostPath(p string) Path {
	if !filepath.IsAbs(p) {
		panic(fmt.Sprintf("bug: expected an absolute host path: %s", p))
	}
	return Path(p)
}

func newpath(p string) Path { return Path(p) }

func (p Path) Dir() Path { return newpath(filepath.Dir(p.String())) }

func (p Path) Base() string { return filepath.Base(p.String()) }

func (p Path) Join(strs ...string) Path {
	parts := make([]string, len(strs)+1)
	parts[0] = p.String()
	for i, str := range strs {
		parts[1+i] = filepath.FromSlash(str)
	}
	return newpath(filepath.Join(parts...))
}

func (p Path) String() string { return string(p) }

func (p Path) HasPrefix(str string) bool { return strings.HasPrefix(p.String(), str) }

func (p Path) TrimPrefix(another Path) string {
	return strings.TrimPrefix(p.String(), another.String())
}
