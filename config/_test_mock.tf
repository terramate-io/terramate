// TERRAMATE: GENERATED AUTOMATICALLY DO NOT EDIT

resource "local_file" "config" {
  content = <<-EOT
package config // import "github.com/terramate-io/terramate/config"

Package config provides high level Terramate configuration facilities.

const DefaultFilename = "terramate.tm.hcl" ...
const ErrScriptSchema errors.Kind = "script config has an invalid schema" ...
const ErrStackValidation errors.Kind = "validating stack fields" ...
const ErrSchema errors.Kind = "config has an invalid schema"
const MaxScriptDescRunes = 1000
const MaxScriptNameRunes = 128
func IsStack(root *Root, dir string) bool
func ReverseStacks(stacks List[*SortableStack])
func Skip(name string) bool
func ValidateWatchPaths(rootdir string, stackpath string, paths []string) (project.Paths, error)
type Assert struct{ ... }
    func EvalAssert(evalctx *eval.Context, cfg hcl.AssertConfig) (Assert, error)
type DirElem interface{ ... }
type Input struct{ ... }
    func EvalInput(evalctx *eval.Context, input hcl.Input) (Input, error)
type Inputs []Input
type List[T DirElem] []T
    func LoadAllStacks(root *Root, cfg *Tree) (List[*SortableStack], error)
    func StacksFromTrees(trees List[*Tree]) (List[*SortableStack], error)
type Output struct{ ... }
    func EvalOutput(evalctx *eval.Context, output hcl.Output) (Output, error)
type Outputs []Output
type Root struct{ ... }
    func LoadRoot(rootdir string) (*Root, error)
    func NewRoot(tree *Tree) *Root
    func TryLoadConfig(fromdir string) (tree *Root, configpath string, found bool, err error)
type Script struct{ ... }
    func EvalScript(evalctx *eval.Context, script hcl.Script) (Script, error)
type ScriptCmd struct{ ... }
type ScriptCmdOptions struct{ ... }
type ScriptJob struct{ ... }
type SortableStack struct{ ... }
type Stack struct{ ... }
    func LoadStack(root *Root, dir project.Path) (*Stack, error)
    func NewStackFromHCL(root string, cfg hcl.Config) (*Stack, error)
    func TryLoadStack(root *Root, cfgdir project.Path) (stack *Stack, found bool, err error)
type Tree struct{ ... }
    func NewTree(cfgdir string) *Tree
EOT

  filename = "${path.module}/mock-config.ignore"
}
