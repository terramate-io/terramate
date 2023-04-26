---
title: Sharing Data - Globals | Terramate
description: Terramate adds powerful capabilities such as code generation, stacks, orchestration, change detection, data sharing and more to Terraform.

prev:
  text: 'Change Detection'
  link: '/change-detection/'

next:
  text: 'Code Generation'
  link: '/code-generation/'
---

# Sharing Data

To keep your code [DRY](https://en.wikipedia.org/wiki/Don%27t_repeat_yourself)
it is important to have an easy and safe way to define data once and share it
across your project.

This is done on Terramate using globals and metadata. Globals are defined by
the user, similar to how you would define locals in Terraform, and metadata
is provided by Terramate.

Terramate globals and metadata are integrated with Terraform using code
generation, you can check it into more details [here](../code-generation/index.md).

# Globals

Globals provide a way to define data that can be reused
across stacks with a clear hierarchical/merge semantic.

Defining globals is fairly straightforward, you just need to
add a **globals** block to your [Terramate configuration file](../configuration/index.md):

```hcl
globals {
  env = "staging"
  values = [ 1, 2, 3 ]
  obj = {
    value = "string"
  }
}
```

The **globals** block also supports setting attributes inside an
existent object. For the globals defined in the above example, the block
below will set additional properties of the `global.obj` variable:

```hcl
globals obj {
  number = 10
}
```

This will make the `global.obj` look like:

```hcl
{
  value = "string"
  number = 10
}
```

In the same way, multiple labels can be provided, and Terramate will
automatically create the object keys as needed to fulfill the user
intent. Example:

```hcl
globals "obj" "child" "grandchild" {
  list = []
}
```

This will make the `global.obj` look like:

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

Globals can reference [metadata](#metadata):

```hcl
globals {
  extended_stack_path = "extended/${terramate.path}"
}
```

And also reference other globals:

```hcl
globals {
  info = "something"
  extended_stack_path = "${global.info}/extended/${terramate.path}"
}
```

Globals can be defined on multiple Terramate files, let's call this
set of files in a specific directory a **configuration**. Given
this definition:

* A project has multiple configurations, one for each of its directories.
* The most specific configuration is the stack directory.
* The most general configuration is the project root directory.
* Globals can't be redefined in the same configuration.
* Globals can be redefined in different configurations.
* Globals can reference globals from other configurations.

Each stack will have its globals defined by loading them from
the stack directory and up until the project root
is reached. This is called the stack globals set.

For globals being redefined on different configurations we follow
a simple merge strategy to build each stack globals set:

* Globals with different names are merged.
* Globals with same names: more specific configuration replaces the general one.

Let's explore a little further with an example.
Given a project structured like this:

```
.
└── stacks
    ├── stack-1
    │   └── terramate.tm.hcl
    └── stack-2
        └── terramate.tm.hcl
```

The configurations available, from more specific to more general, for `stack-1` are:

* stacks/stack-1
* stacks
* . (the project root dir)

To create globals that will be available for all stacks in the entire project
just add a [Terramate configuration file](../configuration/index.md) on the project
root with some useful globals:

```hcl
globals {
  project_name = "awesome-project"
  useful       = "useful"
}
```

Now any stack on the project can reference these globals on their
[Terramate configuration](../configuration/index.md).

Now, let's say one of the stacks wants to add more globals, to do
so we can add globals on the stack configuration by creating the file
`stacks/stack-1/globals.tm.hcl`:

```hcl
globals {
  stack_data = "some specialized stack-1 data"
}
```

Now `stacks/stack-1` globals set is:

```
project_name = "awesome-project"
useful       = "useful"
stack_data   = "some specialized stack-1 data"
```

And for `stacks/stack-2`:

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

Now `stacks/stack-1` globals set is:

```hcl
project_name = "awesome-project"
useful       = "overriden by stack-1"
stack_data   = "some specialized stack-1 data"
```

And for `stacks/stack-2` it remains:

```hcl
project_name = "awesome-project"
useful       = "useful"
```

Overriding happens at the global name level, so objects/maps/lists/sets
won't get merged, they are completely replaced by the most
specific configuration with the same global name.

Let's say we add this to our project root configuration:

```hcl
globals {
  project_name = "awesome-project"
  useful       = "useful"
  object       = { field_a = "field_a", field_b = "field_b" }
}
```

And redefine it on `stacks/stack-1/globals.tm.hcl`:

```hcl
globals {
  object = { field_a = "overriden_field_a" }
}
```

Now `stacks/stack-1` globals set is:

```hcl
project_name = "awesome-project"
useful       = "useful"
object       = { field_a = "overriden_field_a" }
```

And for `stacks/stack-2`:

```hcl
project_name = "awesome-project"
useful       = "useful"
object       = { field_a = "field_a", field_b = "field_b" }
```

It is also possible to **unset** a global by setting it to the value `unset`:

```hcl
globals {
  a = unset
}
```

After a global is unset any access to it will fail just like if the global was
never defined. This affects the global on the whole hierarchy, after unset
the global will be undefined for all child configurations.

It is not allowed to use `unset` in any other context except a direct assignment
to a global.

## Lazy Evaluation

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

* **stack-a** = /stacks/stack-a
* **stack-b** = /stacks/stack-b

### terramate.stack.path.relative (string)

The stack path relative from the project root directory.

Given this project layout:

```
.
└── stacks
    ├── stack-a
    └── stack-b
```

* **stack-a** = stacks/stack-a
* **stack-b** = stacks/stack-b

### terramate.stack.path.basename (string)

The base name of the stack path.

Given this project layout:

```
.
└── stacks
    ├── stack-a
    └── stack-b
```

* **stack-a** = stack-a
* **stack-b** = stack-b

### terramate.stack.path.to\_root (string)

The relative path from the stack to the project root.

Given this project layout:

```
.
└── stacks
    ├── stack-a
    └── stack-b
```

* **stack-a** = ../..
* **stack-b** = ../..

### terramate.stack.id (string)

The ID of the stack as defined on the stack configuration.
If the stack doesn't have a ID defined on its configuration
this metadata will be undefined (no default value provided).

Please consider [stack configuration](../stacks/index.md) to see how
you can define the stack ID.

### terramate.stack.name (string)

The name of the stack as defined on the stack configuration.
If the stack doesn't have a name defined on its configuration
the name will default to `terramate.stack.path.basename`.

Given this stack layout (from the root of the project):

```
.
└── stacks
    ├── stack-a
    └── stack-b
```

* **stack-a** = stack-a
* **stack-b** = stack-b

Please consider [stack configuration](../stacks/index.md) to see how
you can change the default stack name.

### terramate.stack.description (string)

The description of the stack, if it has any.
The default value is an empty string.

Please consider [stack configuration](../stacks/index.md) to see how
you can change the default stack description.

### terramate.stack.tags (list)

The list of stack tags. The default value is an empty list.

Please consider [stack configuration](../stacks/index.md) to see how you can change the stack tags.

## Deprecated

Here is a list of older metadata that still can be used but are in the
process of deprecation.

### terramate.path (string)

Superseded by terramate.stack.path.absolute.

### terramate.name (string)

Superseded by terramate.stack.name.

### terramate.description (string)

Superseded by terramate.stack.description.
