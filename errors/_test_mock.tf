// TERRAMATE: GENERATED AUTOMATICALLY DO NOT EDIT

resource "local_file" "errors" {
  content = <<-EOT
package errors // import "github.com/terramate-io/terramate/errors"

Package errors implements the Terramate standard error type. It's heavily
influenced by Rob Pike `errors` package in the Upspin project:

    https://commandcenter.blogspot.com/2017/12/error-handling-in-upspin.html

func As(err error, target interface{}) bool
func HasCode(err error, code Kind) bool
func Is(err, target error) bool
func IsAnyKind(err error, kinds ...Kind) bool
func IsKind(err error, k Kind) bool
type DetailedError struct{ ... }
    func D(format string, a ...any) *DetailedError
type Error struct{ ... }
    func E(args ...interface{}) *Error
type ErrorDetails struct{ ... }
type Kind string
    const ErrInternal Kind = "terramate internal error"
type List struct{ ... }
    func L(errs ...error) *List
EOT

  filename = "${path.module}/mock-errors.ignore"
}
