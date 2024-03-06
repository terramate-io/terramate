---
title: terramate script info - Command
description: Show detailed info about a Terramate Script using the `terramate script info` command.
---

# Script Info

**Note:** This is an upcoming experimental feature that is subject to change in the future. To use it now, you must enable the project config option `terramate.config.experiments = ["scripts"]`

The `terramate script info` command lists details about all script definitions matching the given LABELs (see [script run](../run) command for details about matching labels). The information provided by `script info` includes:

| Name        | Meaning                                                                     |
| ----------- | --------------------------------------------------------------------------- |
| Definition  | Where the script is defined                                                 |
| Description | The description attribute in the script                                     |
| Stacks      | The stacks within the scope of the script (i.e. those stacks it can run on) |
| Jobs        | The commands that comprise the script                                       |

This information is always relative to the current directory (or the value of `-C`).

**Note: ** Scripts with the same name that are overridden in child directories are considered separate scripts.

## Usage

`terramate script info LABEL...`

## Examples

Show information about a script called "deploy" defined at /scripts.tm.hcl

```bash
$ terramate script info deploy
Definition: /scripts.tm.hcl:1,1-8,2
Description: dummy deploy
Stacks:
  /dev/ec2
  /prd/ec2
  /stg/ec2
Jobs:
  * ["echo","deploying ${global.env}"]
```
