---
title: terramate script info - Command
description: The script info command gives you information about a script

prev:
  text: 'Run'
  link: '/cli/cmdline/run'

next:
  text: 'Script List'
  link: '/cli/cmdline/script-list'
---

# Script Info

**Note:** This is an experimental command that is likely subject to change in the future.

The `script info` command shows details of all scripts matching the command line argument. The details include:

| Name        | Meaning                                |
| ----------- | -------------------------------------- |
| Definition  | Where the script is defined            |
| Description | The description label in the script    |
| Stacks      | The stacks where the script can be run |
| Jobs        | The commands that comprise the script  |

This information is always relative to the current directory (or the value of `-C`).

**Note: ** Scripts with the same name that are overwritten in child directories are considered separate scripts.

## Usage

`terramate experimental script info NAME`

## Examples

Show information about a script called "deploy" defined at /scripts.tm.hcl

```bash
$ terramate-script experimental script-info deploy
Definition: /scripts.tm.hcl:1,1-8,2
Description: dummy deploy
Stacks:
  /dev/ec2
  /prd/ec2
  /stg/ec2
Jobs:
  * ["echo","deploying ${global.tags.env}"]
```
