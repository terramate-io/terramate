---
title: terramate script tree - Command
description: The script tree command shows a tree of all scripts

prev:
  text: 'Script Run'
  link: '/cli/cmdline/script-run'

next:
  text: 'Trigger'
  link: '/cli/cmdline/trigger'
---

# Script Tree

Shows a tree-view of all scripts relative to the current directory (or a specific directory with the -C flag). The tree expands all sub-directories, and the parent path back to the project root, showing script definitions per directory.

## Usage

`terramate experimental script tree`

## Example

```bash
terramate experimental script tree
/
│ * deploy: deploy the infra
├── dev
│   │ * destroy: destroy the infra
│   └── ec2
│         # ec2
│         ~ deploy
│         ~ destroy
├── prd
│   └── ec2
│         # ec2
│         ~ deploy
└── stg
    └── ec2
          # ec2
          ~ deploy
```

Same setup from a leaf directory

```bash
 terramate -C dev/ec2 experimental script tree
/
│ * deploy: deploy the infra
└── dev
    │ * destroy: destroy the infra
    └── ec2
          # ec2
          ~ deploy
          ~ destroy
```
