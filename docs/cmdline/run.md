---
title: terramate run - Command
description: With the terramate run command you can execute any command in a single or a list of stacks.

prev:
  text: 'Stacks'
  link: '/stacks/'

next:
  text: 'Sharing Data'
  link: '/data-sharing/'
---

# Run

The `run` command executes **any command** in a single or a range of stacks following
the orchestration [order of execution](../orchestration/index.md). 

The `run` command allows you to filter for a specific set of stacks such as:
- changed stacks
- stacks with or without specific tags
- stacks in a specific directory

For details on how the change detection and order of execution works in Terramate please see:

- [Change Detection](../change-detection/index.md)
- [Orchestration](../orchestration/index.md)

## Usage

`terramate run [options] CMD`

## Examples

Run a command in all stacks:

```bash
terramate run terraform init
```

Run a command in all stacks inside a specific directory:

```bash
terramate run --chdir stacks/aws -- terraform init
```

Run a command in all stacks that [contain changes](../change-detection/index.md):

```bash
terramate run --changed -- terraform init
```

Run a command in all stacks that contain changes and specific tags:

```bash
terramate run  --changed --tags type:k8s -- kubectl diff
```

Run a command in all stacks that don't contain specific tags, with reversed [order of execution](../orchestration/index.md):

```bash
terramate run  --reverse --no-tags type:k8s -- terraform apply
```

## Options

- `-B, --git-change-base=STRING` Git base ref for computing changes
- `-c, --changed` Filter by changed infrastructure
- `--tags=TAGS` Filter stacks by tags. Use ":" for logical AND and "," for logical OR. Example: --tags `app:prod` filters stacks containing tag "app" AND "prod". If multiple `--tags` are provided, an OR expression is created. Example: `--tags a --tags b` is the same as `--tags a,b`
- `--no-tags=NO-TAGS,...` Filter stacks that do not have the given tags
- `--disable-check-gen-code` Disable outdated generated code check
- `--disable-check-git-remote` Disable checking if local default branch is updated with remote
- `--continue-on-error` Continue executing in other stacks in case of error
- `--no-recursive` Do not recurse into child stacks
- `--dry-run` Plan the execution but do not execute it
- `--reverse` Reverse the order of execution
