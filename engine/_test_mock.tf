// TERRAMATE: GENERATED AUTOMATICALLY DO NOT EDIT

resource "local_file" "engine" {
  content = <<-EOT
package engine // import "github.com/terramate-io/terramate/engine"

Package engine provides the core functionality of Terramate.

const ErrRunFailed errors.Kind = "execution failed" ...
const ErrCurrentHeadIsOutOfDate errors.Kind = "current HEAD is out-of-date with the remote base branch"
func IsDeploymentTask(t StackRunTask) bool
func IsDriftTask(t StackRunTask) bool
func IsPreviewTask(t StackRunTask) bool
func ParseFilterTags(tags, notags []string) (filter.TagClause, error)
type CloudOrgState struct{ ... }
type CloudState struct{ ... }
type Engine struct{ ... }
    func Load(wd string, clicfg cliconfig.Config, uimode UIMode, printers printer.Printers) (e *Engine, found bool, err error)
type GitFilter struct{ ... }
    func NewGitFilter(isChanged bool, gitChangeBase string, enable []string, disable []string) (GitFilter, error)
    func NoGitFilter() GitFilter
type Hooks struct{ ... }
type LogSyncCondition func(task StackRunTask, run StackRun) bool
type LogSyncer func(logger *zerolog.Logger, e *Engine, run StackRun, logs cloud.CommandLogs)
type DependencyFilters struct{ ... }
type Project struct{ ... }
    func NewProject(wd string) (prj *Project, found bool, err error)
type RunAfterHook func(engine *Engine, run StackCloudRun, res RunResult, err error)
type RunAllOptions struct{ ... }
type RunBeforeHook func(engine *Engine, run StackCloudRun)
type RunResult struct{ ... }
type StackCloudRun struct{ ... }
    func SelectCloudStackTasks(runs []StackRun, pred func(StackRunTask) bool) []StackCloudRun
type StackRun struct{ ... }
type StackRunTask struct{ ... }
type UIMode int
    const HumanMode UIMode = iota ...
EOT

  filename = "${path.module}/mock-engine.ignore"
}
