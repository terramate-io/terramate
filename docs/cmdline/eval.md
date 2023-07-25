---
title: terramate eval - Command
description: With the terramate eval command you can fully evaluate a Terramate expression.

prev:
  text: 'Create'
  link: '/cmdline/create'

next:
  text: 'Fmt'
  link: '/cmdline/fmt'
---

# Eval

**Note:** This is an experimental command that is likely subject to change in the future.

The `eval` command allows you to fully evaluate a Terramate expression.

## Usage

`terramate experimental eval EXPRS`

## Examples

Evaluate an expression that returns the uppercase version of the current stack name: 

```bash
terramate experimental eval 'tm_upper(terramate.stack.name)'
```
