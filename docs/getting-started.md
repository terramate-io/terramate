# Getting Started

## Installing

### Using Go

To install using Go just run:

```sh
go install github.com/mineiros-io/terramate/cmd/terramate@<version>
```

Where `<version>` is any terramate [version tag](https://github.com/mineiros-io/terramate/tags),
or you can just install the **latest** release:

```sh
go install github.com/mineiros-io/terramate/cmd/terramate@latest
```

### Using a package manager

- macOS: You can install Terramate on macOS using
  [Homebrew](https://formulae.brew.sh/formula/terramate): `brew install terramate`

### Using Release Binaries

To install Terramate using a release binary, find the
[appropriate package](https://github.com/mineiros-io/terramate/releases) for
your system and download it.

After downloading Terramate, unzip the package. Terramate runs as a single
binary named `terramate`. Any other files in the package can be safely removed
and Terramate will still function.

Finally, make sure that the `terramate` binary is available on your PATH.
This process will differ depending on your operating system.

### Using Docker

If you don't want to install Terramate on your host you can use
[Docker](https://www.docker.com/) or [Podman](https://podman.io/) to
run Terramate inside a container:

```sh
docker run ghcr.io/mineiros-io/terramate
```

Container images tagged with release versions are also provided.
Click [here](https://github.com/mineiros-io/terramate/pkgs/container/terramate/versions)
for a list of the available container image tags.

### Auto Completion

Terramate supports autocompletion of commands for _bash_, _zsh_ and _fish_. To
install the completion just run the command below and open a new shell session:

```sh
terramate install-completions
```

## Project Setup

If you already have a project versioned on Git setting up
Terramate is as easy as just [installing Terramate](#installing).
Terramate comes with sensible defaults so just using it inside a pre existent
Git repository should not require any configurations.

The exception being repositories that have a default remote branch
other than `origin/main`, in that case to make change detection work you will
need to set a customized [project configuration](project-config.md).

If you want to play around with Terramate from scratch locally you can also
setup a local git repository:

```sh
playground=$(mktemp -d)
local_origin=$(mktemp -d)

git init -b main "${playground}"
git init -b main "${local_origin}"  --bare

cd "${playground}"
git remote add origin "${local_origin}"

echo "My Terramate Playground" > README.md

git add README.md
git commit -m "first commit"
git push --set-upstream origin main

# Start using terramate
```

Terramate can also work without any VCS setup, it will only require
a Terramate configuration at the top level directory of the project

```sh
playground=$(mktemp -d)
cd "${playground}"

cat > terramate.tm.hcl <<- EOM
terramate {
  config {
  }
}
EOM

# Start using terramate
```

In a setup with no VCS change detection features will not be available.

You can also check our [live example](https://github.com/mineiros-io/terramate-example-code-generation).
