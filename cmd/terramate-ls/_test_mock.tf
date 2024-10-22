// TERRAMATE: GENERATED AUTOMATICALLY DO NOT EDIT

resource "local_file" "terramate-ls" {
  content = <<-EOT
Terramate-ls is a language server. For details on how to use it just run:

    terramate-ls --help
EOT

  filename = "${path.module}/mock-terramate-ls.ignore"
}
