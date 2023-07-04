---
title: Installation | Terramate
description: Terramate adds powerful capabilities such as code generation, stacks, orchestration, change detection, data sharing and more to Terraform.

prev:
  text: "About Stacks"
  link: "/about-stacks"

next:
  text: "Getting Started"
  link: "/getting-started"
---

# Installation

## Using Go

For installing versions greater than `v0.2.18`, please run:

```sh
go install github.com/terramate-io/terramate/cmd/terramate@<version>
```

For older versions, the command below is required (see [this](https://github.com/golang/go/issues/60452) issue for the reason):

```sh
go install github.com/mineiros-io/terramate/cmd/terramate@<version>
```

Where `<version>` is any terramate [version tag](https://github.com/terramate-io/terramate/tags),
or you can just install the **latest** release:

```sh
go install github.com/terramate-io/terramate/cmd/terramate@latest
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

1. Visit the Terramate [download page](https://terramate.io/download) or locate the suitable package for your system on the [Terramate Releases page](https://github.com/terramate-io/terramate/releases).

2. Download the Terramate package.

3. Unzip the downloaded package to extract the Terramate binary, which is named `terramate`. You can safely remove any other files in the package without affecting Terramate's functionality.

4. Ensure that the `terramate` binary is available on your PATH. The process for this will vary based on your operating system.

## Using Docker

If you prefer not to install Terramate directly on your host system,
you can use either [Docker](https://www.docker.com/) or [Podman](https://podman.io/) to run Terramate within a container.

To do so, execute the following command:

```sh
docker run ghcr.io/terramate-io/terramate
```

We also provide container images tagged with specific release versions.
To view a list of available container image tags, visit this [link](https://github.com/terramate-io/terramate/pkgs/container/terramate/versions).

**Note:** Our image doesn't come with additional dependencies such as Terraform. We recommend building
your own image by using our image as the base image to install additional dependencies required.

We recommend mounting your Terramate project as a Docker mount volume to use it inside the container.
Depending on your setup, you should set the correct permissions when running the container.

```sh
docker run -it -u $(id -u ${USER}):$(id -g ${USER}) \
  -v /some/repo:/some/repo \
  ghcr.io/terramate-io/terramate:latest -C /some/repo --changed run -- cmd
```

## Auto Completion

Terramate supports autocompletion of commands for _bash_, _zsh_ and _fish_. To
install the completion just run the command below and open a new shell session:

```sh
terramate install-completions
```
