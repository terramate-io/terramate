---
title: terramate experimental vendor download - Command
description: Vendor a dependency using the `terramate experimental vendor download` command.
---

# Vendor Download

::: warning
This is an experimental command and is likely subject to change in the future.
:::

The `terramate experimental vendor download` command vendors dependencies such as Terraform modules.

## Usage

`terramate experimental vendor download [options] DEPENDENCY VERSION`

## Examples

Vendor a specific Terraform module version:

```bash
terramate experimental vendor download github.com/mineiros-io/terraform-google-cloud-run v0.2.1
```

## Options

- `--dir=STRING` The directory to download the dependency to.
