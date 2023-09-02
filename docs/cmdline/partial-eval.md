---
title: terramate partial-eval - Command
description: With the terramate partial-eval command you can partially evaluate a Terramate expression.

prev:
  text: 'Metadata'
  link: '/cmdline/metadata'

next:
  text: 'Run Env'
  link: '/cmdline/run-env'
---

# Partial Eval

**Note:** This is an experimental command that is likely subject to change in the future.

The `partial-eval` command allows you to fully evaluate a Terramate expression. The difference to [`eval`](./eval.md) is
that partial eval does not evaluate [functions](../functions/index.md).

## Usage

`terramate experimental partial-eval EXPRS`

## Examples

Evaluate an expression that returns the uppercase version of the current stack name: 

```bash
terramate experimental eval 'tm_upper(terramate.stack.name)'
```
