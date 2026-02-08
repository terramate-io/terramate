// Copyright 2026 Google LLC
// SPDX-License-Identifier: MPL-2.0

stack {
  name        = "package grpc // import \"github.com/terramate-io/terramate/plugin/grpc\""
  description = "package grpc // import \"github.com/terramate-io/terramate/plugin/grpc\"\n\nPackage grpc provides gRPC plugin discovery utilities.\n\nconst HostAddrEnv = \"TM_PLUGIN_HOST_ADDR\"\nfunc DiagnosticsError(diags []*pb.Diagnostic) error\nfunc HCLOptionsFromSchema(pluginName string, binaryPath string, schemas []*pb.HCLBlockSchema) []hcl.Option\nfunc HandshakeConfig() plugin.HandshakeConfig\nfunc IsPluginEnv() bool\nfunc PluginMap(server *Server) map[string]plugin.Plugin\ntype Client struct{ ... }\ntype GRPCPlugin struct{ ... }\ntype HCLExternalData struct{ ... }\ntype HostClient struct{ ... }\n    func NewHostClient(binaryPath string) (*HostClient, error)\n    func NewHostClientWithEnv(binaryPath string, extraEnv []string) (*HostClient, error)\ntype HostService struct{ ... }\n    func NewHostService(root *config.Root, userTerramateDir string) *HostService\ntype InstalledPlugin struct{ ... }\n    func DiscoverInstalled(userTerramateDir string) ([]InstalledPlugin, error)\ntype Server struct{ ... }"
  tags        = ["golang", "grpc", "plugin"]
  id          = "0d380bc7-69ea-4c88-af16-46ded25d000e"
}
