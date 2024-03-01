---
title: terramate generate - Command
description: With the terramate generate command Terramate will generate all files.
---

# Generate

The `generate` command generates files for all code generation strategies. For an overview of code generation strategies available please see the [code generation documentation](../code-generation/index.md).

## Usage

`terramate generate [options]`


## Examples

Generate files and return status code = 2 when files were touched:

```bash
terramate generate --detailed-exit-code
```
