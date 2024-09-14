// TERRAMATE: GENERATED AUTOMATICALLY DO NOT EDIT

resource "local_file" "testserver" {
  content = <<-EOT
Package main implements the cloudmock service.
EOT

  filename = "${path.module}/mock-testserver.ignore"
}
