// TERRAMATE: GENERATED AUTOMATICALLY DO NOT EDIT

resource "local_file" "grpc" {
  content = <<-EOT
package grpc // import "github.com/terramate-io/terramate/plugin/grpc"

Package grpc provides gRPC plugin discovery utilities.

const HostAddrEnv = "TM_PLUGIN_HOST_ADDR"
func DiagnosticsError(diags []*pb.Diagnostic) error
func HCLOptionsFromSchema(pluginName string, binaryPath string, schemas []*pb.HCLBlockSchema) []hcl.Option
func HandshakeConfig() plugin.HandshakeConfig
func IsPluginEnv() bool
func PluginMap(server *Server) map[string]plugin.Plugin
type Client struct{ ... }
type GRPCPlugin struct{ ... }
type HCLExternalData struct{ ... }
type HostClient struct{ ... }
    func NewHostClient(binaryPath string) (*HostClient, error)
    func NewHostClientWithEnv(binaryPath string, extraEnv []string) (*HostClient, error)
type HostService struct{ ... }
    func NewHostService(root *config.Root, userTerramateDir string) *HostService
type InstalledPlugin struct{ ... }
    func DiscoverInstalled(userTerramateDir string) ([]InstalledPlugin, error)
type Server struct{ ... }
EOT

  filename = "${path.module}/mock-grpc.ignore"
}
