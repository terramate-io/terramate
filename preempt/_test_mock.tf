// TERRAMATE: GENERATED AUTOMATICALLY DO NOT EDIT

resource "local_file" "preempt" {
  content = <<-EOT
package preempt // import "github.com/terramate-io/terramate/preempt"

Package preempt implements cooperative scheduling for preemptable functions that
can await keys produced by other functions.

func Await(ctx context.Context, key string) error
func Run(ctx context.Context, fns iter.Seq[Preemptable]) error
type Preemptable func(ctx context.Context) (keys []string, err error)
EOT

  filename = "${path.module}/mock-preempt.ignore"
}
