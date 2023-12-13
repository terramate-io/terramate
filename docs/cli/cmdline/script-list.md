---
title: terramate script list - Command
description: The script list command shows a list of scripts that can be run in the current directory

prev:
  text: 'Script Info'
  link: '/cli/cmdline/script-info'

next:
  text: 'Script Run'
  link: '/cli/cmdline/script-run'
---

# Script List

**Note:** This is an experimental command that is likely subject to change in the future.

Shows a list of all uniquely-named scripts that can be executed with `script run` in the current directory. If there are multiple definitions with the same name, a parent is selected over a child, or a first sibling over a later sibling (ordered by directory name).

## Usage

`terramate experimental script list`
