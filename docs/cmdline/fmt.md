---
title: terramate fmt - Command
description: With the terramate fmt command you can rewrite Terramate configuration files to a canonical format.

prev:
  text: 'Eval'
  link: '/cmdline/eval'

next:
  text: 'Generate'
  link: '/cmdline/generate'
---

# Fmt

The `fmt` command rewrites all Terramate configuration files (.tm.hcl) to a canonical format.
By default, `fmt` scans the current directory for Terramate configuration files recursively.

## Usage

`terramate fmt`

## Examples

Format all files in the current directory recursively:

```bash
terramate fmt
```

Change working directory and list unformatted files only:

```bash
terramate fmt --check --chdir format/in/this/directory
```

## Options

- `--check` Lists unformatted files. Exit with exit code `0` if all is well formatted, `1` otherwise.
