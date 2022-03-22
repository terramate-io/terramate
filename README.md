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
started in a non-intrusive way.

* Avoid duplication by easily sharing data across your project.
* Explicitly define order of execution of stacks.
* Code generation ensures that plan/apply is always done with plain Terraform commands.
* Change detection, including for local modules used by stacks.
* All done with [HCL](https://github.com/hashicorp/hcl).

For more details on how this is achieved you can check:

* [Why stacks ?](docs/why-stacks.md)
* [Change detection](docs/change-detection.md)

## Getting Started

### Installing

To install using Go just run:

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

TODO: More details on the basics to setup a project
