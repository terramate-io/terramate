<!-- mdtocstart -->

# Table of Contents

- [terramate](#terramate)
    - [Installing](#installing)
    - [Why using stacks?](#why-using-stacks)
    - [Detecting IaC changes](#detecting-iac-changes)

<!-- mdtocend -->

# terramate

Terramate is a tool for managing multiple terraform stacks.

The stack concept is not defined by Hashicorp's Terraform tooling but just a
convention used by the _Terraform community_, so a stack can be loosely defined
as:

```
A terraform stack is a runnable terraform module that operates on a subset of
the infrastructure's resource.
```

By _runnable_ it means it has everything needed to call
`terraform init/plan/apply` . For example, a module used by other modules, ie,
don't have the `provider` explicitly set, is not runnable hence it's
**not a stack**.

So, if your runnable terraform module creates your whole infrastructure, *it's
also not a stack*.


## Installing

To install **terramate** using Go just run:

```
go install github.com/mineiros-io/terramate/cmd/terramate@<version>
```

Where **<version>** is any terramate [version tag](https://github.com/mineiros-io/terramate/tags),
or you can just install the **latest** using go install:

```
go install github.com/mineiros-io/terramate/cmd/terramate@latest
```

Or if you have the project cloned locally just run:

```
make install
```

We put great effort into keeping the main branch stable, so it should be safe
to use **latest** to play around, but not recommended for long term automation
since you won't get the same build result each time you run the install command.


## Why using stacks?

The stack concept is advised for several reasons:

- High frequency of infrastructure change

If you infrastructure have a high frequency of change, for example, several
deploys per day/week or several configuration changes per day/week, then if you
apply the change in your single terraform project, some dependent resources can
be recreated leading to increased downtime. There are several reasons why a
dependent module can be recreated, eg.: attributes with variable interpolation
from recreated resources; underspecified attributes refreshing during plan, etc.

By using stacks you can modify only the stacks affected by the deploys or
configuration changes needed, but you have to choose the size of the stack
wisely to avoid duplication.

- Reduce the blast radius

A small change can lead to catastrofic events if you're not careful or makes a
mistake like forgetting a "prevent_destroy" attribute in the production database
lifecycle declaration. Sometime someone will commit mistakes and it's better if
you could reduce the impact of such errors.
An extreme example is: avoiding a database instance destroy because of a dns TTL
change.

- Reduce execution time

By using stacks you can reduce the time a infrastracture change takes to finish.
This is even more appealing if your terraform changes are applied through CI
builders running in the cloud because faster integration/builds leads to reduced
cloud costs.

- Ownership

In the case of a big organization you probably don't want a single person or
team responsible for the whole infrastructure. The company's stacks can be
spread over several repositories and managed by different teams.

By having this separation it also makes it easier when you want separation
by resource permissions, so having some stacks that can only be run by
specific individuals or teams.

- Others

There are lots of other opinionated reasons why stacks would be a good option:
stacks per environments or deploy region location, stacks per topic (IAM vs
actual resources) and so on.

## Detecting IaC changes

When changing your infrastructure (made up of a set of stacks) it's common to
make several changes to several stacks. But now that you have multiple terraform
states (per stack), how to apply the changes only to the affected resources?

The terramate solves this by imposing a workflow:

1. The default branch (commonly main) has the production (applied) code.
2. Before planning and apply, the changes must be committed in a feature/bugfix
  branch.
3. The IaC project must use [non
  fast-forwarded](https://git-scm.com/docs/git-merge#_fast_forward_merge) merge
  commits. (the default in github and bitbucket).

Why this workflow?

By using the method above all commits (except first) in the default branch are
merge commits, then we have an easy way of detecting which stacks in the current
feature branch differs from the main branch.

The technique is super simple and consists of finding the common ancestor
between the current branch and the default branch (most commonly the commit that
current branch forked from) and then list the files that have changed since then
and filter the ones that are part of terraform stacks.
