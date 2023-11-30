---
title: Metadata
description: Metadata provides a set of scoped values in the *terramate* namespace.

prev:
  text: 'Globals'
  link: '/cli/data-sharing/globals.md'

next:
  text: 'Map'
  link: '/cli/map'
---

# Metadata

Terramate provides a set of metadata that can be
accessed through the variable namespace **terramate**.

This can be referenced from any Terramate code to reference
information like the path of the stack or its name.

# Project Metadata

Project metadata is the same independent of stack.

## terramate.version (string)

The Terramate version.

## terramate.stacks.list (string)

List of all stacks inside the project. Each stack is represented by its
absolute path relative to the project root. The list will be ordered
lexicographically.

## terramate.root.path.fs.absolute (string)

The absolute path of the project root directory. Will be the same for all stacks.

## terramate.root.path.fs.basename (string)

The base name of the project root directory. Will be the same for all stacks.


# Stack Metadata

Stack metadata is specific per stack, so each stack will have different values.

## terramate.stack.path.absolute (string)

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

## terramate.stack.path.relative (string)

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

## terramate.stack.path.basename (string)

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

## terramate.stack.path.to\_root (string)

Specifies the relative path from the stack to the project root. Given this project layout:

```
.
└── stacks
    └── stack-a
        └── stack-b
```

* For **stack-a** it returns `../..`
* For **stack-b** it returns `../../..`

## terramate.stack.id (string)

Defines the ID of the stack as stated in the stack configuration. If an ID is not defined in
the stack configuration, this metadata will be undefined (no default value provided).

Refer to [stack configuration](../stacks/index.md) for details on defining stack IDs.

## terramate.stack.name (string)

Specifies the stack's name as defined in the stack configuration. If a name is not defined,
it defaults to `terramate.stack.path.basename`. To change the default stack name, refer to
[stack configuration](../stacks/index.md).

## terramate.stack.description (string)

Describes the stack, if a description is provided. If not, the default value is an empty string.
You can modify the default stack description via the [stack configuration](../stacks/index.md).

## terramate.stack.tags (list)

Represents a list of stack tags. If no tags are defined, the default value is an empty list.

You can update stack tags using the [stack configuration](../stacks/index.md).

# Deprecated

Here is a list of older metadata that still can be used but are in the
process of deprecation.

| S/N  |  Deprecated                    |   Superseded                   |
|------|--------------------------------| -------------------------------|
|  1   | terramate.path (string)        |  terramate.stack.path.absolute |
|  2   | terramate.name (string)        |  terramate.stack.name          |
|  3   | terramate.description (string) |  terramate.stack.description   |
