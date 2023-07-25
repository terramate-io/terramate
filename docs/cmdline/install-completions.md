---
title: terramate install-completions - Command
description: With the terramate install-completions command you can install some handy shell completions for the Terramate CLI.

prev:
  text: 'Globals'
  link: '/cmdline/globals'

next:
  text: 'List'
  link: '/cmdline/list'
---

# Install Completions

The `install-completions` installs autocompletion of commands for _bash_, _zsh_ and _fish_.

## Usage

`terramate install-completions`

## Examples

Install the completions in your environment:

```bash
terramate install-completions
```

Uninstall the completions in your environment:

```bash
terramate install-completions --uninstall
```

## Options

- `--uninstall` Removes the completions from your environment.
