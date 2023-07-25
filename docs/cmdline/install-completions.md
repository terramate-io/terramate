---
title: terramate install-completions - Command
description: With the terramate install-completions command you can install some handy shell completions for the Terramate CLI.

prev:
  text: 'Stacks'
  link: '/stacks/'

next:
  text: 'Sharing Data'
  link: '/data-sharing/'
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
