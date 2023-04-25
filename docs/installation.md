---
title: Installation | Terramate
description: Terramate adds powerful capabilities such as code generation, stacks, orchestration, change detection, data sharing and more to Terraform.

prev:
  text: 'About Stacks'
  link: '/about-stacks'

next:
  text: 'Getting Started'
  link: '/getting-started'
---

# Installation

## Using Go

To install using Go just run:

```sh
go install github.com/mineiros-io/terramate/cmd/terramate@<version>
```

Where `<version>` is any terramate [version tag](https://github.com/mineiros-io/terramate/tags),
or you can just install the **latest** release:

```sh
go install github.com/mineiros-io/terramate/cmd/terramate@latest
```

## Using a package manager

### Brew

You can install Terramate on macOS using [Homebrew](https://formulae.brew.sh/formula/terramate):

`brew install terramate`

### Asdf

You can install Terramate using [asdf](https://asdf-vm.com/):

```
asdf plugin add terramate
asdf install terramate latest
```

## Using Release Binaries

To install Terramate using a release binary, go to the
[download page](https://terramate.io/download) or find the appropriate package in
the [Terramate Releases page](https://github.com/mineiros-io/terramate/releases) for your system.

After downloading Terramate, unzip the package. Terramate runs as a single
binary named `terramate`. Any other files in the package can be safely removed
and Terramate will still function.

Finally, make sure that the `terramate` binary is available on your PATH.
This process will differ depending on your operating system.

## Using Docker

If you don't want to install Terramate on your host you can use
[Docker](https://www.docker.com/) or [Podman](https://podman.io/) to
run Terramate inside a container:

```sh
docker run ghcr.io/mineiros-io/terramate
```

Container images tagged with release versions are also provided.
Click [here](https://github.com/mineiros-io/terramate/pkgs/container/terramate/versions)
for a list of the available container image tags.

## Auto Completion

Terramate supports autocompletion of commands for _bash_, _zsh_ and _fish_. To
install the completion just run the command below and open a new shell session:

```sh
terramate install-completions
```
