// Copyright 2025 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

stack {
  name        = "package engine // import \"github.com/terramate-io/terramate/engine\""
  description = "package engine // import \"github.com/terramate-io/terramate/engine\"\n\nPackage engine provides the core functionality of Terramate.\n\nconst ErrRunFailed errors.Kind = \"execution failed\" ...\nconst ErrCurrentHeadIsOutOfDate errors.Kind = \"current HEAD is out-of-date with the remote base branch\"\nfunc IsDeploymentTask(t StackRunTask) bool\nfunc IsDriftTask(t StackRunTask) bool\nfunc IsPreviewTask(t StackRunTask) bool\nfunc ParseFilterTags(tags, notags []string) (filter.TagClause, error)\ntype CloudOrgState struct{ ... }\ntype CloudState struct{ ... }\ntype Engine struct{ ... }\n    func Load(wd string, clicfg cliconfig.Config, uimode UIMode, printers printer.Printers) (e *Engine, found bool, err error)\ntype GitFilter struct{ ... }\n    func NewGitFilter(isChanged bool, gitChangeBase string, enable []string, disable []string) (GitFilter, error)\n    func NoGitFilter() GitFilter\ntype Hooks struct{ ... }\ntype LogSyncCondition func(task StackRunTask, run StackRun) bool\ntype LogSyncer func(logger *zerolog.Logger, e *Engine, run StackRun, logs cloud.CommandLogs)\ntype OutputsSharingOptions struct{ ... }\ntype Project struct{ ... }\n    func NewProject(wd string) (prj *Project, found bool, err error)\ntype RunAfterHook func(engine *Engine, run StackCloudRun, res RunResult, err error)\ntype RunAllOptions struct{ ... }\ntype RunBeforeHook func(engine *Engine, run StackCloudRun)\ntype RunResult struct{ ... }\ntype StackCloudRun struct{ ... }\n    func SelectCloudStackTasks(runs []StackRun, pred func(StackRunTask) bool) []StackCloudRun\ntype StackRun struct{ ... }\ntype StackRunTask struct{ ... }\ntype UIMode int\n    const HumanMode UIMode = iota ..."
  tags        = ["engine", "golang"]
  id          = "9622aa3b-a9f6-4725-9fd0-8d746fa3ca10"
}
