// TERRAMATE: GENERATED AUTOMATICALLY DO NOT EDIT

resource "local_file" "modvendor" {
  content = <<-EOT
package modvendor // import "github.com/terramate-io/terramate/modvendor"

Package modvendor provides basic functions and types to support Terraform module
vendoring.

func AbsVendorDir(rootdir string, vendorDir project.Path, modsrc tf.Source) string
func SourceDir(path string, rootdir string, vendordir project.Path) string
func TargetDir(vendorDir project.Path, modsrc tf.Source) project.Path
EOT

  filename = "${path.module}/mock-modvendor.ignore"
}
