// TERRAMATE: GENERATED AUTOMATICALLY DO NOT EDIT

resource "local_file" "terramate" {
  content = <<-EOT
Terramate is a tool for managing multiple Terraform stacks. Providing stack
execution orchestration and code generation as a way to share data across
different stacks. For details on how to use it just run:

    terramate --help
EOT

  filename = "${path.module}/mock-terramate.ignore"
}
