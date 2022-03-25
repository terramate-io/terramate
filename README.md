<!-- mdtocstart -->

# Table of Contents

- [Terramate](#terramate)
    - [Getting Started](#getting-started)
        - [Installing](#installing)
            - [Using Go](#using-go)
            - [Using Release Binaries](#using-release-binaries)
            - [Using Docker](#using-docker)
            - [Auto Completion](#auto-completion)
        - [Project Setup](#project-setup)

<!-- mdtocend -->

# Terramate

[![GoDoc](https://pkg.go.dev/badge/github.com/mineiros-io/terramate)](https://pkg.go.dev/github.com/mineiros-io/terramate)
![CI Status](https://github.com/mineiros-io/terramate/actions/workflows/ci.yml/badge.svg)
[![Go Report Card](https://goreportcard.com/badge/github.com/mineiros-io/terramate)](https://goreportcard.com/report/github.com/mineiros-io/terramate)
[![codecov](https://codecov.io/gh/mineiros-io/terramate/branch/main/graph/badge.svg?token=gMRUkVUAQ4)](https://codecov.io/gh/mineiros-io/terramate)

Terramate is a tool for managing multiple Terraform stacks.

The stack concept is not defined by Hashicorp's Terraform tooling but just a
convention used by the _Terraform community_, so a stack can be loosely defined
as:

```
A terraform stack is a runnable terraform module that operates on a subset of
the infrastructure's resource.
```

It provides ways to keep your code [DRY](https://en.wikipedia.org/wiki/Don%27t_repeat_yourself)
and also manage dependencies between stacks with minimal effort to get
started in a non-intrusive way.

* Avoid duplication by easily sharing data across your project.
* Explicitly define order of execution of stacks.
* Code generation ensures that plan/apply is always done with plain Terraform commands.
* Change detection, including for local modules used by stacks.
* Not a wrapper, you keep using Terraform or any other of your tools.
* All done with [HCL](https://github.com/hashicorp/hcl).

For more details on how this is achieved, please consider:

* [Why Stacks ?](docs/why-stacks.md)
* [Change Detection](docs/change-detection.md)
* [Config Overview](docs/config-overview.md)
* [Configuring A Project](docs/project-config.md)
* [Sharing Data](docs/sharing-data.md)
* [Code Generation](docs/codegen/overview.md)
* [Orchestrating Stacks Execution](docs/orchestration.md)

## Getting Started

### Installing

#### Using Go

To install using Go just run:

```sh
go install github.com/mineiros-io/terramate/cmd/terramate@<version>
```

Where **<version>** is any terramate [version tag](https://github.com/mineiros-io/terramate/tags),
or you can just install the **latest** release:

```sh
go install github.com/mineiros-io/terramate/cmd/terramate@latest
```

#### Using Release Binaries

TODO

#### Using Docker

If you don't want to install Terramate on your host you can use
[Docker](https://www.docker.com/) or [Podman](https://podman.io/) to
run Terramate inside a container:

```sh
docker run ghcr.io/mineiros-io/terramate
```

Container images tagged with release versions are also provided.
Click [here](https://github.com/mineiros-io/terramate/pkgs/container/terramate/versions)
for a list of the available container image tags.

#### Auto Completion

Terramate supports autocompletion of commands for *bash*, *zsh* and *fish*. To
install the completion just run the command below and open a new shell session:

```sh
terramate install-completions
```

### Project Setup

TODO: More details on the basics to setup a project
