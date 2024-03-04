---
title: terramate experimental eval - Command
description: Debug Terramate expressions using the `terramate experimental eval` command.
---

# Eval

::: warning
This is an experimental command and is likely subject to change in the future.
:::

The `terramate experimental eval` command allows you to fully evaluate a Terramate expression provided in the arguments.

For security reasons, the filesystem related functions are disabled for now.
Below is the list of functions that **cannot be used** in this command:

- tm_abspath,
- tm_file
- tm_fileexists
- tm_fileset
- tm_filebase64
- tm_filebase64sha256
- tm_filebase64sha512
- tm_filemd5
- tm_filesha1
- tm_filesha256
- tm_filesha512
- tm_templatefile

## Usage

`terramate experimental eval <exprs>`

## Examples

Evaluate an expression that returns the uppercase version of the current stack name:

```sh
terramate experimental eval 'tm_upper(terramate.stack.name)'
```

Evaluate an expression that returns a list of all stacks inside the project:

```sh
terramate experimental eval 'terramate.stacks'
```
