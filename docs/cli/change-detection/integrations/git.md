---
title: Git Change Detection Integration
description: Learn how you can use change detection in Terramate CLI to detect changed stacks.
---

# Git Change Detection Integration

The git change detection integration helps to detect and mark stacks as changed.

## Introduction

The approach is as simple as computing the changed stacks from the changed files
discovered by the `git diff` between the revision of the last
change (ie. the released revision) and the current change.

For the sake of clarity, we'll refer to the released revision as `baseref`, which is an
abbreviation for `base reference`. Usually, this term corresponds to the default branch
(`origin/main` or `origin/default`).

By default, the `baseref` can have two values, depending on if you're in the
default branch or a feature branch, and they are:

* `origin/main`: if you're in a feature branch.
* `HEAD^`: if you're in the default branch.

The [HEAD^](https://git-scm.com/docs/gitrevisions) syntax means the first
parent of the `HEAD` commit and the reasoning for using it for the default
branch is that once you merge, your PR you need to apply the changes in the CI
or locally. If the project adopts a
[non-fast-forward](https://git-scm.com/docs/git-merge#_fast_forward_merge)
merge strategy, every commit—aside from the first one—on the default branch becomes a merge
commit. Utilizing `HEAD^` as the `baseref` enables detection of modifications in the most
recently merged code
Having explained that, hopefully, it becomes clear that change detection in
Terramate works best if the project follows a git flow defined below (by the
way, this is probably the most common git flow used by the git community):

1. The default branch (commonly `main`) is considered to be the stable branch
   that represents the deployed state of your IaC.
2. Changes that should be planned and applied should be added through a feature
   or bugfix branch.
3. The IaC project uses [non fast-forwarded](https://git-scm.com/docs/git-merge#_fast_forward_merge) merge
  commits. (the default in GitHub and Bitbucket).

These are standard in most companies but option 3 is controversial as it
means flows depending on git `rebase` in the `main` branch would not work. If that's the case for
your company, it will require a bit of manual work to apply the changes after merging but alternatively, commands such as
`terraform apply` can be run in the PR's branch just before merging using the default branch base ref (`origin/main`).

## Configuration

The `baseref` can be manually changed by the terramate command line at any given
point in time using the `--git-change-base` option or through the [project configuration](../../projects/configuration.md),
so different strategies for computing the changes are
supported.

If you adopt the rebase merge strategy and need to apply modifications to stacks
affected by the last rebase, it's crucial to first identify the base commit (the commit
before the merge). You can then provide this commit hash in the `--git-change-base` flag to
accomplish the required changes.

```console
$ git branch
main
$ git rev-parse HEAD
80e581a8ce8cc1394da48402cc68a1f47b3cc646
$ git pull origin main
...
$ terramate run --changed --git-change-base 80e581a8ce8cc1394da48402cc68a1f47b3cc646 \
    -- terraform plan
```

`--git-change-base` supports all [git revision](https://git-scm.com/docs/gitrevisions)
syntaxes, so if you know the number of parent commits you can use `HEAD^n` or
`HEAD@{<query>}`, etc.

