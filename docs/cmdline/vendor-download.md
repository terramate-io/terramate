---
title: terramate vendor download - Command
description: With the terramate vendor download command you can vendor a dependency.

prev:
  text: 'Trigger'
  link: '/cmdline/trigger'

next:
  text: 'Version'
  link: '/cmdline/version'
---

# Vendor Download

**Note:** This is an experimental command that is likely subject to change in the future.

The `vendor download` command vendors dependencies such as Terraform modules.

## Usage

`terramate experimental vendor download [options] DEPENDENCY VERSION`

## Examples

Vendor a specific Terraform module version: 

```bash
terramate experimental vendor download github.com/mineiros-io/terraform-google-cloud-run v0.2.1
```

## Options

- `--dir=STRING` The directory to download the dependency to.
