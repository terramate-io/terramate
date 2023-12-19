---
title: Stack Configuration
description: Learn how to configure the metadata and orchestration behavior of stacks in Terramate.
---

# Configure stacks

Stacks can be configured in the `stack {}` block (by default in `stack.tm.hcl`). The options available cover:

- Configuring Metadata (`name`, `description`, `id`, `tags`) for targeting
- Changing the order of execution (`before`, `after`)
- Triggering options (`watch`, `wants`, `wanted_by`)

## Configure metadata

### stack.id (string)(optional)

The stack ID **must** be a string composed of alphanumeric chars + `-` + `_`.
The ID can't be bigger than 64 bytes, **is case insensitive** and
**must** be unique over the whole project.

There is no default value determined for the stack ID, but when users use
the [create](../cmdline/create.md) command to create new stacks or the [clone](../cmdline/clone.md) command to clone stacks,
the ID will default to a [random UUID](https://en.wikipedia.org/wiki/Universally_unique_identifier#:~:text=Version%204%20(random)%5Bedit%5D).

```hcl
stack {
  id = "some_id_that_must_be_unique"
}
```

### stack.name (string)(optional)

The stack name can be any string and defaults to the stack directory base name.

```hcl
stack {
  name = "My Awesome Stack Name"
}
```

### stack.description (string)(optional)

The stack description can be any string and defaults to an empty string.

```hcl
stack {
  description = "My Awesome Stack Description"
}
```

### stack.tags (set(string))(optional)

The tags list must be a unique set of strings where each tag must adhere to the following rules:

- It must start with a lowercase ASCII alphabetic character (`[a-z]`).
- It must end with a lowercase ASCII alphanumeric character (`[0-9a-z]`).
- It must have only lowercase ASCII alphanumeric, `_` and `` characters (`[0-9a-z_-]`).

```hcl
stack {
  ...
  tags = [
    "aws",
    "vpc",
    "bastion",
  ]
}
```

## Configuring the order of execution

::: tip
It's a best practice to use tags instead of paths for defining the order of execution of
stacks with `before` and `after`.
:::

### stack.after (set(string))(optional)

`after` defines a list of stacks that this stack must run after.
It accepts project absolute paths (like `/other/stack`), paths relative to
the directory of this stack (e.g.: `../other/stack`) or a [Tag Filter](../orchestration/index.md#filter-by-tags).

```hcl
stack {
  ...
  after = [
    "tag:prod:networking",
    "/prod/apps/auth"
  ]
}
```

The stack above will run after all stacks tagged with `prod` **and** `networking` and after `/prod/apps/auth` stack.

See the [orchestration docs](../orchestration/index.md#order-of-execution) for details.

### stack.before (set(string))(optional)

Defines the list of stacks that this stack must run `before`, following the same rules as `after`.

```hcl
stack {
  ...
  before = [
    "tag:prod:networking",
    "/prod/apps/auth"
  ]
}
```

## Configure triggering options

### stack.watch (list)(optional)

The list of files that must be watched for changes in the [change detection](../change-detection/index.md).

```hcl
stack {
  ...
  watch = [
    "/policies/mypolicy.json"
  ]
}
```

The configuration above will mark the stack as changed whenever
the file `/policies/mypolicy.json` changes.

### stack.wants (set(string))(optional)

This attribute defines a list of stacks that will be run whenever this stack is run. Example:

```hcl
stack {
  ...
  wants = [
    "/other/stack"
  ]
}
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
It's *very important to note* that adding a stack to `wants` *does not alter the run order*. If you want a dependent
stack to run before or after another you must also use the `before` and `after` attributes.
:::

### stack.wanted_by (set(string))(optional)

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
stack {
  ...
  wanted_by = ["/stacks/stack-a-2"]
}
```
