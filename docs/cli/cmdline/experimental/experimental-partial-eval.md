---
title: terramate experimental partial-eval - Command
description: Debug Terramate expressions using the `terramate experimental partial-eval` command.
---

# Partial Eval

::: warning
This is an experimental command and is likely subject to change in the future.
:::

The `terramate experimental partial-eval` command allows you to partial evaluate a Terramate expression. The difference to [`eval`](./experimental-eval.md) is that only the Terramate variables and Terramate functions are evaluated, all
the rest is left in the expression as is.

Similarly to [experimental eval](./experimental-eval.md), for security reasons the `partial-eval` **does not**
support **filesystem related** [functions](../../code-generation/functions/index.md).
Below is the list of functions **not available** in this command:

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

`terramate experimental partial-eval EXPRS`

## Examples

Evaluate an expression that returns the uppercase version of the current stack name:

```bash
terramate experimental partial-eval '"${var.variable} ${tm_upper(terramate.stack.name)}"'
"${var.variable} MY STACK"
```
