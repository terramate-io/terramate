// TERRAMATE: GENERATED AUTOMATICALLY DO NOT EDIT

resource "local_file" "plugin" {
  content = <<-EOT
package plugin // import "github.com/terramate-io/terramate/plugin"

Package plugin provides plugin installation helpers.

func DefaultUserTerramateDir() (string, error)
func IsCompatible(baseVersion, compatibleWith string) (bool, error)
func ManifestPath(pluginDir string) string
func ParseNameVersion(value string) (string, string)
func PluginDir(userTerramateDir, name string) string
func PluginsDir(userTerramateDir string) string
func Remove(userTerramateDir, name string) error
func ResolveUserTerramateDir() (string, error)
func SaveManifest(pluginDir string, m Manifest) error
func VerifySHA256(path, expected string) error
func VerifySignature(binaryPath, signaturePath, publicKeyPath string) error
type Binary struct{ ... }
type BinaryKind string
    const BinaryCLI BinaryKind = "cli" ...
type InstallOptions struct{ ... }
type Manifest struct{ ... }
    func Install(ctx context.Context, name string, opts InstallOptions) (Manifest, error)
    func InstallFromLocal(_ context.Context, name string, opts InstallOptions) (Manifest, error)
    func ListInstalled(userTerramateDir string) ([]Manifest, error)
    func LoadManifest(pluginDir string) (Manifest, error)
type Protocol string
    const ProtocolGRPC Protocol = "grpc"
type RegistryAsset struct{ ... }
type RegistryBinary struct{ ... }
type RegistryClient struct{ ... }
    func NewRegistryClient(baseURL string) *RegistryClient
type RegistryIndex struct{ ... }
type RegistryVersion struct{ ... }
type Type string
    const TypeGRPC Type = "grpc"
EOT

  filename = "${path.module}/mock-plugin.ignore"
}
