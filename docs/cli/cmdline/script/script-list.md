---
title: terramate script list - Command
description: List available Terramate Scripts using the `terramate script list` command.
---

# Script List

**Note:** This is an upcoming experimental feature that is subject to change in the future. To use it now, you must enable the project config option `terramate.config.experiments = ["scripts"]`

Shows a list of all uniquely-named scripts that can be executed with `script run` in the current directory. If there are multiple definitions with the same name, a parent is selected over a child, or a first sibling over a later sibling (ordered by directory name).

## Usage

`terramate script list`
