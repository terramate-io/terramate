---
title: Sharing Data - Globals
description: Terramate enables you to define data once and distribute it throughout your project. This can be accomplished using globals and metadata.

prev:
  text: 'Change Detection'
  link: '/change-detection/'

next:
  text: 'Code Generation'
  link: '/code-generation/'
---

# Sharing Data in Terramate

Maintaining [DRY](https://en.wikipedia.org/wiki/Don%27t_repeat_yourself)
(Don't Repeat Yourself) principles in your code is crucial to keep your
project neat and manageable. Terramate facilitates this practice by enabling you to define
data once and distribute it throughout your project. This can be accomplished using two
main constructs: Globals and Metadata.

**Globals** are user-defined entities, similar to locals in Terraform, whereas **Metadata** is
information supplied by Terramate itself. These are integrated with Terraform through a code
generation process. To delve deeper into code generation process, refer to [here](../code-generation/index.md).


## Globals

Globals allow you to establish data that can be reused across multiple stacks, using a
hierarchical merge semantics. This ensures consistent and easy data sharing within your
project.

### Defining Globals

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


### Referencing Metadata and other Globals

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

### Usage of Globals across multiple Terramate Files

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

## Working with Globals: An Example

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

### Unsetting Globals

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


## Lazy Evaluation in Terramate

So far, we've described how globals on different configurations are merged.
Given that globals can reference other globals and Terramate metadata, it is
important to be clear about how/when evaluation happens.

Globals are lazily evaluated. The per stack process can
be described in this order:

* Load globals for each configuration, starting on the stack.
* Merge strategy is applied as configurations are loaded.
* All merging is done and the globals set is defined for the stack.
* The stack globals set is evaluated.

This means that globals can reference globals on other configurations
independent of how specific or general the configuration is since it is all
merged together into a single globals set before evaluation.

# Metadata

Terramate provides a set of metadata that can be
accessed through the variable namespace **terramate**.

This can be referenced from any Terramate code to reference
information like the path of the stack or its name.


## Project Metadata

Project metadata is the same independent of stack.

### terramate.version (string)

The Terramate version.

### terramate.stacks.list (string)

List of all stacks inside the project. Each stack is represented by its
absolute path relative to the project root. The list will be ordered
lexicographically.

### terramate.root.path.fs.absolute (string)

The absolute path of the project root directory. Will be the same for all stacks.

### terramate.root.path.fs.basename (string)

The base name of the project root directory. Will be the same for all stacks.


## Stack Metadata

Stack metadata is specific per stack, so each stack will have different values.

### terramate.stack.path.absolute (string)

The absolute path of the stack relative to the project
root directory, not the host root directory. So it is absolute
on the context of the entire project.

Given this project layout:

```
.
└── stacks
    ├── stack-a
    └── stack-b
```

* For **stack-a** it returns `/stacks/stack-a`.
* For **stack-b** it returns `/stacks/stack-b`.

### terramate.stack.path.relative (string)

Specifies the stack's path relative to the project root directory.

Consider this project layout:

```
.
└── stacks
    ├── stack-a
    └── stack-b
```

* For **stack-a** it returns `stacks/stack-a`.
* For **stack-b** it returns `stacks/stack-b`.

### terramate.stack.path.basename (string)

Gives the stack path directory name (or the last component of the stack absolute path).

Consider this project layout:

```
.
└── stacks
    ├── stack-a
    └── stack-b
```

In this case,

* For **stack-a** it returns `stack-a`.
* For **stack-b** it returns `stack-b`.

### terramate.stack.path.to\_root (string)

Specifies the relative path from the stack to the project root. Given this project layout:

```
.
└── stacks
    └── stack-a
        └── stack-b
```

* For **stack-a** it returns `../..`
* For **stack-b** it returns `../../..`

### terramate.stack.id (string)

Defines the ID of the stack as stated in the stack configuration. If an ID is not defined in
the stack configuration, this metadata will be undefined (no default value provided).

Refer to [stack configuration](../stacks/index.md) for details on defining stack IDs.

### terramate.stack.name (string)

Specifies the stack's name as defined in the stack configuration. If a name is not defined,
it defaults to `terramate.stack.path.basename`. To change the default stack name, refer to
[stack configuration](../stacks/index.md).

### terramate.stack.description (string)

Describes the stack, if a description is provided. If not, the default value is an empty string.
You can modify the default stack description via the [stack configuration](../stacks/index.md).

### terramate.stack.tags (list)

Represents a list of stack tags. If no tags are defined, the default value is an empty list.

You can update stack tags using the [stack configuration](../stacks/index.md).

## Deprecated

Here is a list of older metadata that still can be used but are in the
process of deprecation.

| S/N  |  Deprecated                    |   Superseded                   |
|------|--------------------------------| -------------------------------|
|  1   | terramate.path (string)        |  terramate.stack.path.absolute |
|  2   | terramate.name (string)        |  terramate.stack.name          |
|  3   | terramate.description (string) |  terramate.stack.description   |
