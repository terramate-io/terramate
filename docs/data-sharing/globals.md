---
title: Globals
description: Globals are variables that can be shared across the filesystem tree.

prev:
  text: 'Sharing Data - Overview'
  link: '/data-sharing/overview.md'

next:
  text: 'Code Generation'
  link: '/code-generation/'
---

# Globals

Globals allow you to establish data that can be reused across multiple stacks, using a
hierarchical merge semantics. This ensures consistent and easy data sharing within your
project.

# Defining Globals

Globals are added to your [Terramate configuration file](../configuration/index.md) within a `globals` block.
Here's an example:

```hcl
globals {
  env = "staging"
  values = [ 1, 2, 3 ]
  obj = {
    value = "string"
  }
}
```

A globals block also permits setting attributes within an existing object. To illustrate, the
following block appends properties to the `global.obj` variable defined above:

```hcl
globals obj {
  number = 10
}
```

After this, `global.obj` would appear as:

```hcl
{
  value = "string"
  number = 10
}
```

Multiple labels can be provided, and Terramate will automatically establish the object keys as
necessary. Here's an example:

```hcl
globals "obj" "child" "grandchild" {
  list = []
}
```

This will make the `global.obj` appear as:

```hcl
{
  value = "string"
  number = 10
  child = {
    grandchild = {
      list = []
    }
  }
}
```

Additionally, the [map](../map.md) block is supported inside the `globals` block
for building complex objects.

The **globals** can be referenced on the [code generation](../code-generation/index.md):

```hcl
generate_hcl "backend.tf" {
  content {
    backend "type" {
      param = "value-${global.env}"
    }
  }
}

globals {
  env = "staging"
}
```


# Referencing Metadata and other Globals

```hcl
globals {
  extended_stack_path = "extended/${terramate.path}"
}
```

They can also reference other globals:

```hcl
globals {
  info = "something"
  extended_stack_path = "${global.info}/extended/${terramate.path}"
}
```

# Usage of Globals across multiple Terramate Files

Globals can be defined across multiple Terramate files, with the set of files in a
specific directory referred to as a **configuration**. Following this terminology:

* A project has multiple configurations, one for each of its directories.
* The most specific configuration is the stack directory.
* The most general configuration is the project root directory.
* Globals can't be redefined in the same configuration.
* Globals can be redefined in different configurations.
* Globals can reference globals from other configurations.

Each stack will have its globals defined by loading them from
the stack directory and up until the project root
is reached. This is called the stack globals set.

When globals are redefined across different configurations, a simple merge strategy is
adopted:

* Globals with different names are merged.
* For globals with identical names, the more specific configuration replaces the general one.

# Working with Globals: An Example

Consider a project with the following structure:

```
.
└── stacks
    ├── stack-1
    │   └── terramate.tm.hcl
    └── stack-2
        └── terramate.tm.hcl
```

For stack-1, the available configurations, listed from most specific to most general, are:

* stacks/stack-1
* stacks
* . (the project root dir)

To create globals that will be available for all stacks in the entire project
just add a [Terramate configuration file](../configuration/index.md) in the project
root with some useful globals:

```hcl
globals {
  project_name = "awesome-project"
  useful       = "useful"
}
```

Now any stack in the project can reference these globals in their
[Terramate configuration](../configuration/index.md).

Suppose one of the stacks, stack-1, wants to introduce more globals. This can be done by
adding globals to the stack configuration in the file 'stacks/stack-1/globals.tm.hcl':

```hcl
globals {
  stack_data = "some specialized stack-1 data"
}
```

With this change, the globals set for 'stacks/stack-1' becomes:

```
project_name = "awesome-project"
useful       = "useful"
stack_data   = "some specialized stack-1 data"
```

For 'stacks/stack-2', the globals remain:

```
project_name = "awesome-project"
useful       = "useful"
```

Now, let's redefine a global on `stacks/stack-1`.
We do that by changing `stacks/stack-1/globals.tm.hcl`:

```hcl
globals {
  useful     = "overriden by stack-1"
  stack_data = "some specialized stack-1 data"
}
```

This modification changes `stacks/stack-1` globals set to:

```hcl
project_name = "awesome-project"
useful       = "overriden by stack-1"
stack_data   = "some specialized stack-1 data"
```

For `stacks/stack-2`, the globals set remains unchanged:

```hcl
project_name = "awesome-project"
useful       = "useful"
```

Overriding is conducted at the global name level, meaning objects, maps, lists, and sets
are not merged but entirely replaced by the most specific configuration with the same global name.

For instance, suppose we add this to our project root configuration:

```hcl
globals {
  project_name = "awesome-project"
  useful       = "useful"
  object       = { field_a = "field_a", field_b = "field_b" }
}
```

And redefine `object` in 'stacks/stack-1/globals.tm.hcl':

```hcl
globals {
  object = { field_a = "overriden_field_a" }
}
```

The `stacks/stack-1` globals set becomes:

```hcl
project_name = "awesome-project"
useful       = "useful"
object       = { field_a = "overriden_field_a" }
```

For `stacks/stack-2`, it remains:

```hcl
project_name = "awesome-project"
useful       = "useful"
object       = { field_a = "field_a", field_b = "field_b" }
```

# Unsetting Globals

To unset a global, assign the value `unset` to it:

```hcl
globals {
  a = unset
}
```

Upon unsetting, any access to the global will fail, as if the global was never defined.
This behavior affects the global throughout the entire hierarchy, leaving it undefined for all child configurations.

It's essential to note that `unset` can only be used in direct assignments to a global.
It is not allowed in any other context.
