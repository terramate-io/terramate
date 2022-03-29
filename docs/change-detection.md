# Change Detection

When changing your infrastructure (made up of a set of stacks) it's common to
make several changes to several stacks. But now that you have multiple terraform
states (per stack), how to apply the changes only to the affected resources?
Keep in mind that we don't want to just blindly execute _plan/apply_ to the
unchanged stacks to reduce the blast radius.

We solve that by leveraging the power of the VCS (Version Control System)
already in place. At the moment, Terramate only supports `git` but other VCSs
can be added in the future.

The approach is as simple as computing the changed stacks from the changed files 
discovered by the `git diff` between the revision of the last `terraform applied` 
change (ie. the released revision) and the current change.

Let's call the released revision `baseref`, which means `base reference` which
commonly is the default branch (`origin/main` or `origin/default`).

By default the `baseref` can have two values, depending on if you're in the
default branch or in a feature branch, and they are:

* `origin/main` : if you're in a feature branch.
* `HEAD^` : if you're in the default branch.

The [HEAD^](https://git-scm.com/docs/gitrevisions) syntax means the first
parent of the `HEAD` commit and the reasoning for using it for the default
branch is that once you merged your PR you need to apply the changes in the CI 
or locally. So if the project uses
[non-fast-forwarded](https://git-scm.com/docs/git-merge#_fast_forward_merge)
all commits (except first) in the default branch are merge commits, then by
using `HEAD^` as baseref we can detect changes of the last merged code.

Having explained that, hopefully it becomes clear that change detection in
Terramate works best if the project follows a git flow defined below (by the
way, this is probably the most common git flow used by the git community):

1. The default branch (commonly `main`) is considered to be the stable branch
   that represents the deployed state of your IaC.
2. Changes that should be planned and applied should be added through a feature
   or bugfix branch.
3. The IaC project uses [non
  fast-forwarded](https://git-scm.com/docs/git-merge#_fast_forward_merge) merge
  commits. (the default in GitHub and Bitbucket).

These are standard on most companies but the option 3 is controversial as it
means flows depending on git `rebase` would not work. If that's the case for
your company, it will require a bit of manual work to apply the changes after
merged but alternatively the terraform plan/apply can be run in the PR's branch
just before merge using the default branch base ref (`origin/main`).

The `baseref` can be manually changed by the terramate command line at any given
point in time using the `--git-change-base` option or through the [project configuration](project-config.md),
so different strategies for computing the changes are supported.

Then, if you use rebase as the merge strategy and need to apply the changes to
the stacks modified by the last rebase, you first need to identify the base
commit (the commit before the merge) and then provide this commit hash in the
`--git-change-base` flag.

```
$ git branch
main
$ git rev-parse HEAD
80e581a8ce8cc1394da48402cc68a1f47b3cc646
$ git pull origin main
...
$ terramate run --changed --git-change-base 80e581a8ce8cc1394da48402cc68a1f47b3cc646 \
    -- terraform plan
```

The option also supports all [git
revision](https://git-scm.com/docs/gitrevisions) syntaxes, so if you know the
number of parent commits you can use `HEAD^n` or `HEAD@{<query>}, etc.
