---
title: terramate script info - Command
description: The script info command gives you information about a script

prev:
  text: 'Run'
  link: '/cli/cmdline/run'

next:
  text: 'Script List'
  link: '/cli/cmdline/script/list'
---

# Script Info

**Note:** This is an experimental command that is likely subject to change in the future.

The `script info LABEL...` command lists details about all script definitions matching the given LABELs (see [script run](./run) command for details about matching labels). The information provided by `script info` includes:

| Name        | Meaning                                                                     |
| ----------- | --------------------------------------------------------------------------- |
| Definition  | Where the script is defined                                                 |
| Description | The description attribute in the script                                     |
| Stacks      | The stacks within the scope of the script (i.e. those stacks it can run on) |
| Jobs        | The commands that comprise the script                                       |

This information is always relative to the current directory (or the value of `-C`).

**Note: ** Scripts with the same name that are overridden in child directories are considered separate scripts.

## Usage

`terramate experimental script info LABEL...`

## Examples

Show information about a script called "deploy" defined at /scripts.tm.hcl

```bash
$ terramate experimental script info deploy
Definition: /scripts.tm.hcl:1,1-8,2
Description: dummy deploy
Stacks:
  /dev/ec2
  /prd/ec2
  /stg/ec2
Jobs:
  * ["echo","deploying ${global.env}"]
```
