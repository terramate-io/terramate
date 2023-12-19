---
title: Run Commands in Stacks
description: Learn how to orchestrate the execution of commands in stacks with the terramate run command.
---

# Run Commands in Stacks

Terramate CLI allows you to orchestrate the execution of stacks by running
commands in all stacks or filtering stacks using certain criteria.

## Run commands in all stacks

Running commands in stacks sequentially can be done with the
[run](../cmdline/run.md) command.

**Example:** Run commands in all stacks with `terramate run`

```hcl
terramate run <cmd>
```

::: tip
The [`run-order`](../cmdline/run-order.md) command returns a list that describes the order of execution of your stacks.
:::

## Run commands in a subset of stacks using filter

There are three main ways to filter stacks targeted in `terramate run`:
**scope**, **tags** and **change detection**.

### Filter by scope

Terramate uses the current directory it is being executed to filter out stacks,
i.e., limit the scope of the execution. So if you execute `terramate` from the
project's root directory, all stacks will be selected and change to inner
directories in the project structure will select only stacks that are children
of the current directory.

The `-C <path>` flag can be used to change the scope without having to `cd` to the directory.

**Example:** Change the scope to `some/dir`

```hcl
terramate run -C some/dir -- <cmd>
```

### Filter by tags

Stacks can also be tagged to allow for further targeting.

```hcl
stack {
  name = "Some Application"
  tags = ["kubernetes"]
  id   = "f2b426b2-f614-4fa5-8f12-af78e5dcc13e"
}
```

Tags can be used to filter stacks on any command using `--tags` (or `--no-tags`
to exclude). Logical **AND** and **OR** can be achieved with the `:` and `,` operators.

**Example:** Run a command in all stacks tagged with `kubernetes` or `k8s`.

```hcl
terramate run --tags kubernetes,k8s -- <cmd>
```

### Filter for changed stacks

The `--changed` flag will filter by stacks that have changed in Git compared to a base ref
using the [Git integration](../change-detection/integrations/git.md) of the
[change detection](../change-detection/index.md).

**Example:** Execute a command in all stacks that contain changes

```hcl
terramate run --changed -- <cmd>
```

::: info
The default base ref is `origin/main` when working in a feature branch and
`HEAD^` when on main. The base ref can be changed in the project configuration
(or with `-B`), but the defaults allow for the most common workflow where all
changed stacks in a feature branch should be previewed in a PR and applied on merge.
:::

::: tip
Terramate supports importing code with `import` blocks to allow for code re-use,
and when the source of one of these `import` blocks changes, all stacks where
the code is imported will be marked as changed.
:::

It is possible to monitor files that are outside the stack for changes
using the `watch` property in the configuration of a stack.

**Example:** Watch for changes outside the current stack 

```hcl
stack {
  name        = "Some Application"
  tags        = ["kubernetes"]
  id          = "f2b426b2-f614-4fa5-8f12-af78e5dcc13e"
  watch       = [
    "/path/to/file",
  ]
}
```

### Changing the run scope and order

Sometimes, you want to change the [default order of execution](./index.md#order-of-execution), which can be done
with the [`before`](../stacks/configuration.md#stackbefore-setstringoptional) and
[`after`](../stacks/configuration.md#stackafter-setstringoptional) attributes in the configuration of a stack.
For details, please see the [stacks configuration](../stacks/configuration.md#configuring-the-order-of-execution) documentation.

By default, Terramate will run against all child stacks selected by the filter
(e.g., all changed stacks with `--changed`). It is possible to add explicit
dependencies for stacks not beneath the current directory by using
[`watch`](../stacks/configuration.md#stackwatch-listoptional),
[`wants`](../stacks/configuration.md#stackwants-setstringoptional) and
[`wanted_by`](../stacks/configuration.md#stackwanted_by-setstringoptional) attributes.
