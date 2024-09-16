// TERRAMATE: GENERATED AUTOMATICALLY DO NOT EDIT

resource "local_file" "fs" {
  content = <<-EOT
package fs // import "github.com/terramate-io/terramate/fs"

Package fs provides filesystem related functionality.

func CopyAll(dstdir, srcdir string) error
func CopyDir(destdir, srcdir string, filter CopyFilterFunc) error
func ListTerramateDirs(dir string) ([]string, error)
func ListTerramateFiles(dir string) (filenames []string, err error)
type CopyFilterFunc func(path string, entry os.DirEntry) bool
EOT

  filename = "${path.module}/mock-fs.ignore"
}
