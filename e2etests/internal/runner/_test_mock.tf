// TERRAMATE: GENERATED AUTOMATICALLY DO NOT EDIT

resource "local_file" "runner" {
  content = <<-EOT
package runner // import "github.com/terramate-io/terramate/e2etests/internal/runner"

Package runner provides helpers for compiling and running the Terramate binary
with the intent of doing e2e tests. Additionally, it also provides functions for
building and installing dependency binaries.

var HelperPath string
var HelperPathAsHCL string
var TerraformTestPath string
var TerraformVersion string
func AssertRun(t *testing.T, got RunResult)
func AssertRunResult(t *testing.T, got RunResult, want RunExpected)
func BuildTerramate(projectRoot, binDir string) (string, error)
func BuildTestHelper(projectRoot, binDir string) (string, error)
func InstallTerraform(preferredVersion string) (string, string, func(), error)
func RemoveEnv(environ []string, names ...string) []string
func Setup(projectRoot string) (err error)
func Teardown()
type CLI struct{ ... }
    func NewCLI(t *testing.T, chdir string, env ...string) CLI
    func NewInteropCLI(t *testing.T, chdir string, env ...string) CLI
type Cmd struct{ ... }
type RunExpected struct{ ... }
type RunResult struct{ ... }
EOT

  filename = "${path.module}/mock-runner.ignore"
}
