// TERRAMATE: GENERATED AUTOMATICALLY DO NOT EDIT

resource "local_file" "telemetry" {
  content = <<-EOT
package telemetry // import "github.com/terramate-io/terramate/cmd/terramate/cli/telemetry"

var DefaultRecord = NewRecord()
func Endpoint() url.URL
func GenerateOrReadSignature(cpsigfile, anasigfile string) (string, bool)
func GenerateSignature() string
func ReadSignature(p string) string
func SendMessage(msg *Message, p SendMessageParams) <-chan error
type AuthType int
    const AuthNone AuthType = iota ...
    func DetectAuthTypeFromEnv(credpath string) AuthType
type Message struct{ ... }
type MessageOpt func(msg *Message)
    func BoolFlag(name string, flag bool, ifCmds ...string) MessageOpt
    func Command(cmd string) MessageOpt
    func DetectFromEnv(cmd string, credfile, cpsigfile, anasigfile string) MessageOpt
    func StringFlag(name string, flag string, ifCmds ...string) MessageOpt
type PlatformType int
    const PlatformLocal PlatformType = iota ...
    func DetectPlatformFromEnv() PlatformType
type Record struct{ ... }
    func NewRecord() *Record
type SendMessageParams struct{ ... }
EOT

  filename = "${path.module}/mock-telemetry.ignore"
}
