// TERRAMATE: GENERATED AUTOMATICALLY DO NOT EDIT

resource "local_file" "stdlib" {
  content = <<-EOT
package stdlib // import "github.com/terramate-io/terramate/stdlib"

Package stdlib implements the Terramate language functions.

const ErrTomlDecode errors.Kind = "failed to decode toml content"
const TomlExperimentName = "toml-functions"
func AbspathFunc(basedir string) function.Function
func Functions(basedir string, experiments []string) map[string]function.Function
func HCLDecode() function.Function
func HCLEncode() function.Function
func HCLExpressionFunc() function.Function
func Name(name string) string
func NoFS(basedir string, experiments []string) map[string]function.Function
func Regex() function.Function
func TernaryFunc() function.Function
func TomlDecode() function.Function
func TomlEncode() function.Function
func TryFunc() function.Function
func VendorFunc(basedir, vendordir project.Path, stream chan<- event.VendorRequest) function.Function
func VersionMatch() function.Function
EOT

  filename = "${path.module}/mock-stdlib.ignore"
}
