---
title: Global Variables
description: Learn how you can use Global Variables in Terramate to define data that can be used across multiple stacks.
---

# Global Variables

Globals allow you to define data that can be reused across multiple stacks, using hierarchical merge semantics.
This ensures consistent and easy data sharing within your Terramate project. Globals are often used in
[code generation](../index.md) to e.g. configure the Terraform version used in stacks programmatically.

## Define Globals

Terramate Globals can be defined on any level of the hierarchy using a `globals` block definition:

```hcl
globals {
  <variable-name> = <expression>
}
```

Terramate globals are available in the `global` namespace via `global.<variable-name>` and can be part of any expression.

Terramate Variables (`let`, `global`, and `terramate` namespaces) and all [Terramate Functions](../functions/index.md)
are supported on the right side of the definition of a global variable.

Globals can be defined across multiple Terramate files, with the set of files in a specific directory referred to as a
**configuration**. Following this terminology:

- A project has multiple configurations, one for each of its directories.
- The most specific configuration is the stack directory.
- The most general configuration is the project root directory.
- Globals can't be redefined in the same configuration.
- Globals can be redefined in different configurations.
- Globals can reference globals from other configurations.
- Globals are evaluated only in stack context and do not need to be fully defined on higher levels, i.e., they can
  reference another global that is not yet defined but will be defined on lower levels. For details please see
  [Lazy evaluation](#lazy-evaluation).

When globals are redefined across different configurations, a simple merge strategy is adopted:

- Globals with different names are merged.
- For globals with identical names, the more specific configuration replaces the general one.

## Unset a Global

A global variable can be removed from the definition by assigning it the special value `unset`.

```hcl
globals {
  no_longer_available = unset
}
```

Upon unsetting, any access to the global will fail, as if the global was never defined.

This behavior affects the global throughout the entire hierarchy, leaving it undefined for all child configurations.

It's essential to note that `unset` can only be used in direct assignments to a global. It is not allowed in any other context.

## Labeled Globals

As Terramate Globals are inherited through the hierarchy setting values in complex structures like maps is desired for
some use cases. To set a specific value of a key within a `map` without redefining all keys in the map, the `globals` block supports labels to define what keys to set or replace.

```hcl
globals <variable> [key] [key] ... {
  <key> = <expression>
}
```

Example:

```hcl
globals "mymap" "nested" {
  key = "value"      # set global.mymap.nested.key = "value"
}

# equal initial definition without labels:
globals {
  mymap = {          # set global.mymap = { nested = { key = "value" } }
    nested = {       # set global.mymap.nested = { key = "value" }
      key = "value"  # set global.mymap.nested.key = "value"
    }
  }
}
```

The following evaluation rules are applied and available:

- Any number of labels can be added to a `globals` block to set a nested value within a simple map or any level in maps of maps definitions.
- Labeled globals are evaluated in order of precision: First all unlabeled globals are set, then globals with one label, followed by globals with two labels, etc.
- When assigning labels not yet defined in a map or nested map, the map will be initialized and no previous definition is needed to define an empty map.
- When trying to set a key within a previously defined conflicting type like string or list, an error is raised.

Labeled Globals can be used to populate maps through the hierarchy and use inheritance to define keys only ones on higher levels inheriting them to multiple stacks.

Using `globals "tags" { env = "production" }` can help implement a natural tagging strategy for cloud resources without
maintaining or redefining complex tags in multiple locations.

## Lazy evaluation

Given that globals can reference other globals and metadata, it is important to be clear about how and when evaluation happens.

Globals are lazily evaluated, which means globals block can reference variables that will exist at a later point in time.
For example, a non-stack directory can have globals referencing stack metadata.

Below is a high-level overview of how globals evaluation is implemented for a given stack:

- The globals blocks are loaded from all Terramate files in the stack directory.
- The multiple globals blocks are merged into a single globals definition.
- Recursively, the parent directories have their globals blocks loaded and merged into the same single globals definition
  but giving preference for the Global defined lower in the filesystem tree.
- Then resulting single `globals {}` block has its attributes evaluated.
  - The `terramate.*` namespace is set with the stack [metadata](./metadata.md) values.
  - Globals that depend on other globals are postponed until all dependencies are evaluated.

This means that globals can reference globals in other configurations independent of how specific or general the
configuration is since it is all merged into a single set of globals before evaluation.

## Debugging Globals

To see all globals available in each stack, the `terramate debug globals` command can be used. For details, please see the
[debug globals](../../cmdline/debug/show/debug-show-globals.md) command.
