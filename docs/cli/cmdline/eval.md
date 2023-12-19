---
title: terramate eval - Command
description: With the terramate eval command you can fully evaluate a Terramate expression.
---

# Eval

::: warning
This is an experimental command and is likely subject to change in the future.
:::

The `eval` command allows you to fully evaluate a Terramate expression.

## Usage

`terramate experimental eval EXPRS`

## Examples

Evaluate an expression that returns the uppercase version of the current stack name:

```sh
terramate experimental eval 'tm_upper(terramate.stack.name)'
```

Evaluate an expression that returns a list of all stacks inside the project:
```sh
terramate experimental eval 'terramate.stacks'
```
