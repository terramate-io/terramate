---
title: Context-based Variables
description: Learn how to use the lets block to define context-based, temporary variables that can be used in code generation blocks.
---

# Context-based Variables

Terramate Lets Variables represent context-based variables that can be used to compute local blocks available in the
current code generation block only to not pollute the global namespace with temporary or intermediate variables.

Available contexts are:

- within [`generate_hcl`](../generate-hcl.md) blocks
- within [`generate_file`](../generate-file.md) blocks

They are defined the same way as [Global Variables](./globals.md) and support similar features.

```hcl
lets {
  <variable-name> = <expression>
}
```

The following key differences exist compared to Terramate Globals:

- they are only available in some defined context e.g., `generate_hcl` and `generate_file`
- they do not support labels
- they support the additional `let` namespace in expressions

Example:

```hcl
generate_file "file.json" {
  lets {
    # let.json is available in the current generate_file block only
    json = tm_jsonencode({ "hello" = "world" })
  }

  content = let.json
}
```
