---
title: terramate generate - Command
description: Run the Code Generation for your project by using `terramate generate` command.
---

# Generate

The `terramate generate` command generates files for all code generation strategies. For an overview of code generation strategies available please see the [code generation documentation](../code-generation/index.md).

## Usage

`terramate generate [options]`

## Examples

Generate files and return status code = 2 when files were touched:

```bash
terramate generate --detailed-exit-code
```
