---
title: terramate script tree - Command
description: Dump a tree of available Terramate Scripts using the `terramate script tree` command.
---

# Script Tree

**Note:** This is an upcoming experimental feature that is subject to change in the future. To use it now, you must enable the project config option `terramate.config.experiments = ["scripts"]`

Shows a tree-view of all scripts relative to the current directory (or a specific directory with the -C flag). The tree expands all sub-directories, and the parent path back to the project root, showing script definitions per directory.

## Usage

`terramate script tree`

## Example

```bash
$ terramate script tree
/
│ * deploy: "deploy the infra"
├── dev
│   └── #ec2
│         ~ deploy
├── prd
│   └── #ec2
│         ~ deploy
└── stg
    └── #ec2
          ~ deploy
```
