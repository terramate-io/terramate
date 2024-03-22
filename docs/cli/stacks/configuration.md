---
title: Stack Configuration
description: Learn how to configure the metadata and orchestration behavior of stacks in Terramate.
---

# Configure stacks

Terramate detects stacks based on the existence of a `stack {}` block. The name of the file is not important and can be different from `stack.tm.hcl`. There can be exactly one stack block defined in a stack.

- Define Stack Metadata: `name`, `description`, `id`, `tags`.
- Set an explicit Order of Execution: `before`, `after`.
- Configure Forced Execution: `watch`, `wants`, `wanted_by`.

```hcl
stack {
  id          = "7b5f4d89-70a7-42f0-972f-3be8550e65df"
  name        = "My Awesome Stack Name"
  description = "My Awesome Stack Description"
  tags = [
    "aws",
    "vpc",
    "bastion",
  ]
}
```

## General Stack Metadata

### `id`

The stack ID **must** be a `string` composed of alphanumeric chars, dashes, and underscores.
The ID can't be bigger than 64 bytes, **is case insensitive** and **must** be unique over the whole project. It is required when synchronizen data to Terramate Cloud.

It is recommended to use a lowercase [UUIDv4](<https://en.wikipedia.org/wiki/Universally_unique_identifier#:~:text=Version%204%20(random)%5Bedit%5D>) as stack ID as this is the default when a stack is created using the [`terramate create`](../cmdline/create.md) command.

When stacks are [cloned](../cmdline/experimental/experimental-clone.md) a new UUIDv4 is generated for cloned stacks.

When `id` is missing in stacks, the `terramate create --ensure-id` command can be used to add a UUIDv4 to stacks that did not define an `id` yet.

Example:

```hcl
id = "7b5f4d89-70a7-42f0-972f-3be8550e65df"
```

The id is available as `terramate.stack.id` variable in Code Generation.

### `name`

The optional stack name can be any string. It is supposed to give the stack a human-readable Name.

If not set, it defaults to the basename of the stack path.

The stack name will be synchronized to Terramate Cloud and shown in addition to the stack path to identify a stack.

```hcl
name = "My Awesome Stack Name"
```

The name is available as `terramate.stack.name` variable in Code Generation.

### `description`

The stack description can be any string and can include multiple lines.

It will be synchronized to Terramate Cloud and shown in the Stacks Details area.

```hcl
description = "My Awesome Stack Description"
```

The description is available as `terramate.stack.description` variable in Code Generation.

### `tags`

The tags list is a set of strings and each tag needs to be a lowercase alphanumeric string that can also contain dashes and underscores.

Tags can be used to target/filter stacks in various commands and shall be used when defining order of execution of a stack.

Terramate Cloud allows to filter stacks by tags.

```hcl
tags = [
  "aws",
  "vpc",
  "bastion",
]
```

Tags are available as `terramate.stack.tags` variable in Code Generation and can be used to conditionally generate code for stacks having or not having specific stacks defined.

Examples of commands with tags support:

- Listing having or not having tags set:

  - `terramate list --tags a`
  - `terramate list --no-tags b`

- Running any command in stacks having or not having tags set:

  - `terramate run --tags c -- echo "hi from stack with tag c"`
  - `terramate run --no-tags d -- echo "hi from stack without tag d"`

- Running `my script` Terramate Script in stacks having or not having tags set:

  - `terramate script run --tags e my script`
  - `terramate script run --no-tags f my script`

## Explicit Order of Execution

::: tip
It's a best practice to use tags instead of paths for defining the order of execution of
stacks with `before` and `after`.
:::

### `after`

`after` defines a list of stacks that this stack must run after.
It accepts project absolute paths (like `/other/stack`), paths relative to
the directory of this stack (e.g.: `../other/stack`) or a [Tag Filter](../orchestration/index.md#filter-by-tags).

```hcl
after = [
  "tag:prod:networking",
  "/prod/apps/auth",
]
```

The stack above will run after all stacks tagged with `prod` **and** `networking` and after `/prod/apps/auth` stack.

See the [orchestration docs](../orchestration/index.md#order-of-execution) for details.

### `before`

Defines the list of stacks that this stack must run `before`, following the same rules as `after`.

```hcl
before = [
  "tag:prod:networking",
  "/prod/apps/auth",
]
```

## Influence Change Detection

### `watch`

The list of files that must be watched for changes in the [change detection](../change-detection/index.md).

```hcl
watch = [
  "/policies/mypolicy.json"
]
```

The configuration above will mark the stack as changed whenever
the file `/policies/mypolicy.json` changes.

## Forced Execution

::: warning
Using forced execution is an anti pattern and will lead to a bigger blast radius.
Consider combining the stacks for easier maintainability instead.
Im most scenarios defining Order of Execution is sufficient to guarantee changes are applied in order.
:::

### `wants`

This attribute defines a list of stacks that will be run whenever this stack is run. Example:

```hcl
wants = [
  "/other/stack"
]
```

This can be useful to force a dependency relationship between stacks - for example, if a dependent stack is outside of
the scope (the nested directory hierarchy). E.g., if we had the following hierarchy:

```txt
/stacks
  /stack-a
    /stack-a-1
    /stack-a-2
  /stack-b
```

Running `terramate -C stacks/stack-a` would run `stack-a`, `stack-a-1` and `stack-a-2`.

But if `stack-a-2` always requires stack-b to run, we could put `wants = ["/stacks/stack-b"]` into the `stack-a-2`
configuration and it would always be added to the execution list whenever `stack-a-2` was targeted.

::: info
It's _very important to note_ that adding a stack to `wants` _does not alter the run order_. If you want a dependent
stack to run before or after another you must also use the `before` and `after` attributes.
:::

### `wanted_by`

This attribute is similar to `stack.wants` but works reversely. That is, using the same hierarchy as above, we could
achieve the same result (including `stack-b` whenever we trigger `stack-a-2`) by putting
`wanted_by = ["/stacks/stack-a-2"]` in the stack-b configuration.

```txt
/stacks
  /stack-a
    /stack-a-1
    /stack-a-2
  /stack-b
```

```hcl
# stack-b/stack.tm.hcl
wanted_by = ["/stacks/stack-a-2"]
```
