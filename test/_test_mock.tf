// TERRAMATE: GENERATED AUTOMATICALLY DO NOT EDIT

resource "local_file" "test" {
  content = <<-EOT
package test // import "github.com/terramate-io/terramate/test"

Package test provides testing routines reused throughout terramate code base.

const Username = "terramate tests" ...
func AppendFile(t testing.TB, dir string, filename string, content string)
func AssertChmod(t testing.TB, fname string, mode fs.FileMode)
func AssertConfigEquals(t *testing.T, got, want []config.Assert)
func AssertDiff(t *testing.T, got, want interface{}, msg ...interface{})
func AssertEqualPaths(t *testing.T, got, want project.Path, fmtargs ...any)
func AssertEqualPos(t *testing.T, got, want info.Pos, fmtargs ...any)
func AssertEqualRanges(t *testing.T, got, want info.Range, fmtargs ...any)
func AssertEqualSets[T comparable](t *testing.T, got, want []T)
func AssertFileContentEquals(t *testing.T, fname string, want string)
func AssertFileEquals(t *testing.T, filepath1, filepath2 string)
func AssertGenCodeEquals(t *testing.T, got string, want string)
func AssertStackImports(t *testing.T, rootdir string, stackHostPath string, want []string)
func AssertStacks(t testing.TB, got, want config.Stack)
func AssertTerramateConfig(t *testing.T, got, want hcl.Config)
func AssertTreeEquals(t *testing.T, dir1, dir2 string)
func CanonPath(t testing.TB, path string) string
func Chmod(fname string, mode fs.FileMode) error
func DoesNotExist(t testing.TB, dir, fname string)
func EmptyRepo(t testing.TB, bare bool) string
func FixupRangeOnAsserts(dir string, asserts []config.Assert)
func Getwd(t testing.TB) string
func IsDir(t testing.TB, dir, fname string)
func IsFile(t testing.TB, dir, fname string)
func LookPath(t *testing.T, file string) string
func Mkdir(t testing.TB, base string, name string) string
func MkdirAll(t testing.TB, path string)
func MkdirAll2(t testing.TB, path string, perm fs.FileMode)
func NewExpr(t *testing.T, expr string) hhcl.Expression
func NewGitWrapper(t testing.TB, wd string, env []string) *git.Git
func NewRepo(t testing.TB) string
func NonExistingDir(t testing.TB) string
func ParseSource(t *testing.T, source string) tf.Source
func ParseTerramateConfig(t *testing.T, dir string) hcl.Config
func PrependToPath(env []string, dir string) ([]string, bool)
func ReadDir(t testing.TB, dir string) []os.DirEntry
func ReadFile(t testing.TB, dir, fname string) []byte
func RelPath(t testing.TB, basepath, targetpath string) string
func RemoveAll(t testing.TB, path string)
func RemoveFile(t testing.TB, dir, fname string)
func Symlink(t testing.TB, oldname, newname string)
func TempDir(t testing.TB) string
func WriteFile(t testing.TB, dir string, filename string, content string) string
func WriteRootConfig(t testing.TB, rootdir string)
EOT

  filename = "${path.module}/mock-test.ignore"
}
