// TERRAMATE: GENERATED AUTOMATICALLY DO NOT EDIT

resource "local_file" "grpcplugin" {
  content = <<-EOT
Package main provides a test gRPC plugin binary for e2e coverage.
EOT

  filename = "${path.module}/mock-grpcplugin.ignore"
}
