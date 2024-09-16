// TERRAMATE: GENERATED AUTOMATICALLY DO NOT EDIT

resource "local_file" "event" {
  content = <<-EOT
package event // import "github.com/terramate-io/terramate/event"

Package event implements a simple event stream and defines all events generated
by different parts of Terramate.

type Stream[T any] chan T
    func NewStream[T any](buffsize int) Stream[T]
type VendorProgress struct{ ... }
type VendorRequest struct{ ... }
EOT

  filename = "${path.module}/mock-event.ignore"
}
