// TERRAMATE: GENERATED AUTOMATICALLY DO NOT EDIT

resource "local_file" "helper" {
  content = <<-EOT
helper is a utility command that implements behaviors that are useful when
testing terramate run features in a way that reduces dependencies on the
environment to run the tests.
EOT

  filename = "${path.module}/mock-helper.ignore"
}
