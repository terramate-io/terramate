# Change Detection

When changing your infrastructure (made up of a set of stacks) it's common to
make several changes to several stacks. But now that you have multiple terraform
states (per stack), how to apply the changes only to the affected resources?

Terramate comes with support for change detection by imposing the following workflow:

1. The default branch (commonly `main`) is considered to be the stable branch that represents the deployed state of our IaC.
2. Before planning and apply, the changes must be committed in a feature/bugfix
  branch.
3. The IaC project must use [non
  fast-forwarded](https://git-scm.com/docs/git-merge#_fast_forward_merge) merge
  commits. (the default in github and bitbucket).

## Why this workflow?

By using the method above all commits (except first) in the default branch are
merge commits, then we have an easy way of detecting which stacks in the current
feature branch differs from the main branch.

The technique is super simple and consists of finding the common ancestor
between the current branch and the default branch (most commonly the commit that
current branch forked from) and then list the files that have changed since then
and filter the ones that are part of terraform stacks.
