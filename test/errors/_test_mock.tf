// TERRAMATE: GENERATED AUTOMATICALLY DO NOT EDIT

resource "local_file" "errors" {
  content = <<-EOT
package errors // import "github.com/terramate-io/terramate/test/errors"

Package errors provides useful assert functions for handling errors on tests

func Assert(t *testing.T, err, target error, args ...interface{})
func AssertAsErrorsList(t *testing.T, err error)
func AssertErrorList(t *testing.T, err error, targets []error)
func AssertIsErrors(t *testing.T, err error, targets []error)
func AssertIsKind(t *testing.T, err error, k errors.Kind)
func AssertKind(t *testing.T, got, want error)
EOT

  filename = "${path.module}/mock-errors.ignore"
}
