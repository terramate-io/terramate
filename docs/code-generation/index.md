---
title: Code Generation | Terramate
description: Terramate adds powerful capabilities such as code generation, stacks, orchestration, change detection, data sharing and more to Terraform.

prev:
  text: 'Map Block'
  link: '/map'

next:
  text: 'Generate HCL'
  link: '/code-generation/generate-hcl'
---

# Code Generation

Code generation is the main way you can glue [Terramate data](../data-sharing/index.md)
to other tools, like Terraform.

Different code generation strategies will be provided in the future to support
each integration scenario in the best way possible.

Currently, we support:

* [HCL generation](./generate-hcl.md) with `stack` context.
> This strategy generates code that is closely tied to and relative to the stack where it is defined.
It provides access to a rich set of code generation features, including globals, metadata, functions, lets,
and assertions. By using these features you can easily manipulate and transform data within stack context.
>

* [File generation](./generate-file.md) with `root` and `stack` context.
 > With this strategy, you can generate code at the project root level, outside of specific stacks. It offers 
 access to project metadata, functions, lets, and assertions. BY using this relatively broader context you 
 can generate code that spans multiple stacks or interacts with data at the project level.
 >

# Generation Context

Code generation supports two execution contexts:

- **stack**: generates code relative to the stack where it's defined.
- **root**: generates code outside of stacks.

The `stack` context gives access to all code generation features, such as:

* [Globals](../data-sharing/index.md#globals)
* [All Metadata](../data-sharing/index.md#metadata)
* [Functions](../functions/index.md)
* [Lets](#lets)
* [Assertions](#assertions)

But the `root` context gives access to:

* [Project Metadata](../data-sharing/index.md#project-metadata)
* [Functions](../functions/index.md)
* [Lets](#lets)

By default, the generation `context` is set to stack. However, in the case of the `generate_file` block, you have the flexibility to explicitly change the `context` by specifying `context = root` within the block. This empowers you to generate code that goes beyond the limitations of individual `stacks`.

Example:

```hcl
generate_file "/file.txt" {
    context = root
    content = "something"
}
```

# Labels

All code generation blocks use labels to identify the block and define where
the generated code will be saved but they have different constraints depending
on the generation context.

### 1. `Stack` Context Labels Constraints:

* The labels are in relative path and they must adhere to the format `<dir>/<filename>` or simply `<filename>`, that represents a relative path.
* Regardless of the operating system, labels should be defined using the forward slash `/`.
* Labels should not contain `../`, as code generation is confined to the stack and cannot navigate outside.
* Labels should not start with `./`.
* Labels should not represent symbolic links.
* Each label must be unique within the stack hierarchy for blocks with `condition=true`.
* It can’t be the stack.

### 2. `Root` Context Labels Constraints:

* Labels should follow the format `/<dir>/<filename>` or `/<filename>`, representing an absolute path.
* Similar to stack context, labels should be defined using the forward slash `/`, independent of the operating system.
* Labels should not contain `../`, as code generation is limited to the project root and cannot traverse beyond.
* Labels should not represent symbolic links.
* Each label must be unique within the hierarchy for blocks with `condition=true`.
* It can’t be the stack.


# Lets

The `lets` block can be used to define local scoped variables inside the
generate blocks. Each `generate_*` block has its own `lets` scope that
has access to **globals** and **terramate** namespaces. The symbols defined
in the block are discarded after the block is executed.

The syntax for defining variables is the same as **globals** with the
exception that `lets` does not support labels in the block.

After the block is evaluated, it's values are available in the `let`
namespace.

```hcl
generate_file "sum.txt" {
  lets {
    sum = tm_sum(tm_concat(global.a, global.b))
  }

  content = tm_format("the sum is %d", let.sum)
}
```

# Assertions

Assertions can be used in order to fail code generation for one or more stacks
if some pre-condition is not met, helping to catch mistakes in your configuration.

Assertions can be only used when the generation context is of type `stack`.

All fields can contain expressions accessing **globals**, **lets** and **metadata**.

```hcl
assert {
  assertion = global.a == global.b
  message   = "assertion failed, details: ${global.a} != ${global.b}"
}
```

### Assertions consist of the following fields:

* **assertion**: This obligatory field evaluates a boolean expression that determines the validity of the assertion.

* **message**: Another obligatory field, this evaluates to a string and serves as a descriptive error message that aids in understanding the cause of the assertion failure.

* **warning (optional)**: By default, this field is set to false. However, when evaluated to true, it allows code generation to proceed even in the presence of a false assertion. While code will be generated, a warning highlighting the failed assertion will be displayed during the code generation process.

These fields can contain expressions accessing **globals, lets, and metadata**. When the assertion is false on the context of a stack, code generation for that stack will fail and the reported error will be the one provided on the message field. The stack won't be modified, and no files will be created, modified, or deleted. Assertions offer hierarchical behavior, enabling you to define assert blocks at different levels of the project hierarchy.

Assert blocks can also be defined inside `generate_hcl` and `generate_file` blocks.
When inside one of those blocks it has the same semantics as describe above, with
the exception that it will have access to locally scoped data like the `let` namespace.
