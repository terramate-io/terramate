---
title: How to Install
description: Learn how to install Terramate with Go or package managers such as brew. You can also run Terramate with Docker by building on top of our pre-configured image.
---

# Installation

To get started with Terramate, the first step you need to take is to install [Terramate CLI](https://github.com/terramate-io/terramate),
which distributes two binaries:

- `terramate`: The CLI tool used to create, manage and orchestrate your IaC stacks and workflows.
- `terramate-ls`: The Terramate Language Server to integrate Terramate into your IDE.

## Using a package manager

### Brew

You can install Terramate on macOS using [Homebrew](https://formulae.brew.sh/formula/terramate):

`brew install terramate`

### Asdf

You can install Terramate using [asdf](https://asdf-vm.com/):

```sh
asdf plugin add terramate
asdf install terramate latest
```

## Using release binaries

To install Terramate using a release binary, follow these steps:

1. Visit the [Terramate Releases page](https://github.com/terramate-io/terramate/releases).

2. Download the appropriate package for your operating system and architecture.

3. Decompress the package somewhere on your disk. _Note: all files in the package other than `terramate` and `terramate-ls` can be safely deleted_.

4. Ensure that the `terramate` and `terramate-ls` binaries are available to your `PATH`.
   The process for this will vary based on your operating system.

## Using Go

For installing Terramate with Go, please run:

```sh
go install github.com/terramate-io/terramate/cmd/...@<version>
```

Where `<version>` is any Terramate [version tag](https://github.com/terramate-io/terramate/tags),
or you can just install the **latest** release:

```sh
go install github.com/terramate-io/terramate/cmd/...@latest
```

The commands above install both `terramate` and `terramate-ls` into
your Go binary folder (usually `$HOME/go/bin`).

## Using docker

If you prefer not to install Terramate directly on your host system,
you can use either [Docker](https://www.docker.com/) or [Podman](https://podman.io/) to run Terramate within a container.

To do so, execute the following command:

```sh
docker run ghcr.io/terramate-io/terramate
```

We also provide container images tagged with specific release versions.
To view a list of available container image tags, visit this [link](https://github.com/terramate-io/terramate/pkgs/container/terramate/versions).

::: info
Our image doesn't come with additional dependencies such as Terraform. We recommend
building your own image by using our image as the base image to install additional
dependencies required.
:::

We recommend mounting your Terramate project as a Docker mount volume to use it inside the container.
Depending on your setup, you should set the correct permissions when running the container.

```sh
docker run -it -u $(id -u ${USER}):$(id -g ${USER}) \
  -v /some/repo:/some/repo \
  ghcr.io/terramate-io/terramate:latest -C /some/repo --changed run -- cmd
```

## Auto completion

Terramate supports the autocompletion of commands for _bash_, _zsh_ and _fish_. To
install the completion just run the command below and open a new shell session:

```sh
terramate install-completions
```
