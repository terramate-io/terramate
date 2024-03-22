---
title: Run Commands in Stacks
description: Learn how to orchestrate the execution of commands in stacks with the terramate run command.
---

# Run any Commands in Stacks

Terramate CLI allows you to orchestrate the execution of stacks by running
commands in all stacks or filtering stacks using certain criteria.

Terramate is not limited to executing `terraform` inside of stacks but can execute any command available. This includes but is not limited to `terragrunt`, `tofu`, `kubectl`, `helm`, and `make`.

When running commands in stacks the [defined order of execution](../stacks/configuration#explicit-order-of-execution) is honored and stacks are run in order.

## Run commands in all stacks

Running commands in stacks sequentially can be done with the
[terramate run](../cmdline/run.md) command.

**Example:** Run hello world commands in all stacks with `terramate run`

```hcl
terramate run echo "hello world"
```

## Run commands in selected stacks

When `terramate run` is executed it will run in all stacks that are reachable from the working directory.

The following filters can be used to select a subset of stacks to execute commands in. They can be combined to limit the number of stacks executed in a single run.

### Filter by directory subtree

It is possible to execute Terramate in a subtree of your repository by either changing the working directory into the subdirectory or by temporarily changing the working directory during execution using the `--chdir <path>` command line option (short: `-C <path>`).

```hcl
terramate --chdir path/to/tree -- echo "hello from subtree"
```

### Filter a specific stack

When selecting a specific stack using the `--chdir` command line option the selected stack and all nested stacks will be selected. To only execute the parent stack, using the `--no-recursive` command line option will ensure, that no child stacks will be executed.

```hcl
terramate --chdir path/to/parent-stack --no-recursive -- echo "hello from stack"
```

### Filter by tags

When [tags are defined on stacks](../stacks/configuration#tags), this information can be used to execute commands in stacks with or without specific tags.

```hcl
terramate --tags    k8s,kubernetes -- echo "hello from k8s stack"
terramate --no-tags k8s,kubernetes -- echo "hello from non k8s stack"
```

### Filter for changed stacks

Terramate integrates with various tools to enable [Change Detection](../change-detection/index.md).

Making use of Change Detection features when running commands can improve run-times on local machines and in automation.

To enable it add the `--changed` command line option.

**Example:** Execute a command in all stacks that contain changes

```hcl
terramate run --changed -- echo "hello from changed stack"
```

## Influence the order of execution

Terramate honors the explicit and implicit order of execution when running commands.

`terramate list --run-order` or `terramate run --dry-run` can be used to preview the order in which commands will be executed in stacks.

### Run in parallel

Stacks that are not affected by a specific order of execution can be executed in parallel.

Terramate will always guarantee that ordered stacks will still run in order but independent stacks or stacks that have their depending stacks completed can run in parallel.

By default, Terramate will always execute all stacks in sequence one stack at a time.

**Example:** Run multiple stacks in parallel

```hcl
terramate run --parallel 100 -- echo "hello from stack in parallel"
```

::: warning
It is not possible to run `terraform init` in parallel when provider caching is enabled via `TF_PLUGIN_CACHE_DIR` as Terraform does not support this mode of operation at this time.
:::

### Reverse the run order

The order defined in the configuration of a stack and defined via integrations can be reverted when executing commands in stacks.

**Example:** Execute a command in all stacks but in reverse order

```hcl
terramate run --reverse -- echo "hello from stack in reversed order"
```

This is useful when running destructive operations where dependent stacks need to remove their configuration before other stacks.

An example use-case is the destroy operation of Terraform to destroy stacks in opposite order:

```hcl
terramate run --reverse -- terraform destroy
```
