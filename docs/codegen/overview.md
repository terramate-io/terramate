# Code Generation

Code generation is the main way you can glue [Terramate data](../sharing-data.md)
to other tools, like Terraform.

Different code generation strategies will be provided in the future to support
each integration scenario in the best way possible. 

Currently, we support:

* [HCL generation](./generate-hcl.md) with stack [context](#generation-context).
* [File generation](./generate-file.md) with `root` and `stack` [context](#generation-context).

# Generation Context

Code generation supports two execution contexts:

- stack: generates code relative to the stack where it's defined.
- root: generates code outside of stacks.

The `stack` context gives access to all code generation features, like:

* [Globals](../sharing-data.md#globals)
* [All Metadata](../sharing-data.md#metadata)
* [Functions](../functions.md)
* [Lets](#lets)
* [Assertions](#assertions)

But the `root` context gives access to:

* [Project Metadata](../sharing-data.md#project-metadata)
* [Functions](../functions.md)
* [Lets](#lets)

If not specified the default generation context is `stack`.
The `generate_hcl` block doesn't support changing the `context`, it will always be
of type `stack`. The `generate_file` block supports the `context` attribute which you can explicit change to `root`.
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
on the [generation context](#generation-context).

For `stack` context, the labels must follow the constraints below:

* It is a relative path in the form `<dir>/<filename>` or just `<filename>`
* It is always defined with `/` independent on the OS you are working on
* It does not contain `../` (code can only be generated inside the stack)
* It does not start with `./`
* It is not a symbolic link
* It is not a stack
* It is unique on the whole hierarchy of a stack for all blocks with condition=true.

For `root` context, the constraints are:

* It is an absolute path in the form `/<dir>/<filename>` or just `/<filename>`.
* It is always defined with `/` independent on the OS you are working on
* It does not contain `../` (code can only be generated inside the project root)
* It is not a symbolic link
* It is not a stack
* It is unique on the whole hierarchy for all blocks with condition=true.

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

Assertions can be only used when the [generation context](#generation-context)
is of type `stack` and it has the following fields:

* **assertion** : Obligatory, must evaluate to boolean
* **message** : Obligatory, must evaluate to string
* **warning** : Optional (default=false), must evaluate to boolean

All fields can contain expressions accessing **globals**, **lets** and **metadata**.

```hcl
assert {
  assertion = global.a == global.b
  message   = "assertion failed, details: ${global.a} != ${global.b}"
}
```

When the **assertion** is false on the context of a stack, code generation for
that stack will fail and the reported error will be the one provided on the
**message** field. The stack won't be touched, no files will be changed/created/deleted.

Optionally the **warning** field can be defined and if it is evaluated to true
then an false **assertion** will **not** generate an error. Code will be generated,
but a warning output will be shown during code generation.

The **assert** block has hierarchical behavior, any assert blocks defined in a
directory will be applied to all stacks inside this directory. For example, an
**assert** block defined on the root of a project will be applied to all stacks
in the project.

Assert blocks can also be defined inside `generate_hcl` and `generate_file` blocks.
When inside one of those blocks it has the same semantics as describe above, with
the exception that it will have access to locally scoped data like the `let` namespace.
