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

To install Terramate using a release binary, follow these steps:

1. Visit the Terramate [download page](https://terramate.io/download) or locate the suitable 
package for your system on the [Terramate Releases page](https://github.com/mineiros-io/terramate/releases).

2. Download the Terramate package.

3. Unzip the downloaded package to extract the Terramate binary, which is named `terramate`. 
You can safely remove any other files in the package without affecting Terramate's functionality.

4. Ensure that the `terramate` binary is available on your PATH. The process for this will 
vary based on your operating system.

## Using Docker

If you prefer not to install Terramate directly on your host system, 
you can use either Docker or Podman to run Terramate within a container.

To do so, execute the following command:

```sh
docker run ghcr.io/mineiros-io/terramate
```

We also provide container images tagged with specific release versions. 
To view a list of available container image tags, visit this [link](https://github.com/mineiros-io/terramate/pkgs/container/terramate/versions).

## Auto Completion

Terramate supports autocompletion of commands for _bash_, _zsh_ and _fish_. To
install the completion just run the command below and open a new shell session:

```sh
terramate install-completions
```
