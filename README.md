<!-- mdtocstart -->

# Table of Contents

- [Terramate](#terramate)
    - [Installing](#installing)
    - [Why using stacks?](#why-using-stacks)
        - [High frequency of infrastructure change](#high-frequency-of-infrastructure-change)
        - [Reduce the blast radius](#reduce-the-blast-radius)
        - [Reduce execution time](#reduce-execution-time)
        - [Ownership](#ownership)
        - [Others](#others)
    - [Detecting IaC changes](#detecting-iac-changes)
        - [Why this workflow?](#why-this-workflow)

<!-- mdtocend -->

# Terramate

[![GoDoc](https://pkg.go.dev/badge/github.com/mineiros-io/terramate)](https://pkg.go.dev/github.com/mineiros-io/terramate)
![CI Status](https://github.com/mineiros-io/terramate/actions/workflows/ci.yml/badge.svg)
[![Go Report Card](https://goreportcard.com/badge/github.com/mineiros-io/terramate)](https://goreportcard.com/report/github.com/mineiros-io/terramate)
[![codecov](https://codecov.io/gh/mineiros-io/terramate/branch/main/graph/badge.svg?token=gMRUkVUAQ4)](https://codecov.io/gh/mineiros-io/terramate)

Terramate is a tool for managing multiple Terraform stacks.
It provides ways to keep your code [DRY](https://en.wikipedia.org/wiki/Don%27t_repeat_yourself)
and also manage dependencies between stacks with minimal effort to get
started and very non-intrusive


## Getting Started

### Installing

To install **terramate** using Go just run:

```
go install github.com/mineiros-io/terramate/cmd/terramate@<version>
```

Where **<version>** is any terramate [version tag](https://github.com/mineiros-io/terramate/tags),
or you can just install the **latest** release:

```
go install github.com/mineiros-io/terramate/cmd/terramate@latest
```

Terramate supports autocompletion of commands for *bash*, *zsh* and *fish*. To
install the completion just run the command below and open a new shell session:

```
$ terramate install-completions
```

If you don't want to install Terramate on your host you can use
[Docker](https://www.docker.com/) or [Podman](https://podman.io/) to
run Terramate inside a container:

```
docker run ghcr.io/mineiros-io/terramate
```

Container images tagged with release versions are also provided.

### Project Setup

TODO: More details


## Configuring Your Project

Checkout the docs on [overall configuration](docs/config.md) for details
on how different kinds of configurations can help you to avoid duplication
and write solid Infrastructure as Code.


## Change Detection

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

### Why this workflow?

By using the method above all commits (except first) in the default branch are
merge commits, then we have an easy way of detecting which stacks in the current
feature branch differs from the main branch.

The technique is super simple and consists of finding the common ancestor
between the current branch and the default branch (most commonly the commit that
current branch forked from) and then list the files that have changed since then
and filter the ones that are part of terraform stacks.
