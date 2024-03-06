---
title: terramate fmt - Command
description: Ensure formatting of Terramate configuration by using the `terramate fmt` command.
---

# Fmt

The `terramate fmt` command rewrites all Terramate configuration files (.tm.hcl) to a canonical format.
By default, `terramate fmt` scans the current directory for Terramate configuration files recursively.

## Usage

`terramate fmt [options]`

## Examples

Format all files in the current directory recursively:

```bash
terramate fmt
```

Format all files and return status code = 2 if files were formatted:

```bash
terramate fmt --detailed-exit-code
```

(DEPRECATED) Change working directory and list unformatted files only:

```bash
# deprecated
terramate fmt --check --chdir format/in/this/directory
```

## Options

- `--check` Lists unformatted files. Exit with exit code `0` if all is well formatted, `1` otherwise.
- `--detailed-exit-code` Return an appropriate exit code (0 = ok, 1 = error, 2 = no error but changes were made)
