# Code Generation

Code generation is the main way you can glue [Terramate data](../sharing-data.md)
to other tools, like Terraform.

Different code generation strategies will be provided in the future to support
each integration scenario in the best way possible. Currently, we support:

* [HCL generation](./generate-hcl.md)
* [File generation](./generate-file.md)

Some features are available to all code generation strategies, like:

* [Globals](../sharing-data.md#globals)
* [Metadata](../sharing-data.md#metadata)
* [Functions](../functions.md)
* [Assertions](#assertions)

# Assertions

Assertions can be used in order to fail code generation for one or more stacks
if some pre-condition is not met, helping to catch mistakes in your configuration.

It has the following field:

* **assertion** : Obligatory, must evaluate to boolean
* **message** : Obligatory, must evaluate to string
* **warning** : Optional (default=false), must evaluate to boolean

All fields can contain expressions accessing **globals** and **metadata**.

```hcl
assert {
  assertion = global.a == global.b
  message   = "assertion failed, details: ${global.a} != ${global.b}"
}
```

When the **assertion** is false on the context of a stack, code generation for
that stack will fail and the reported error will be the one provided on the
**message** field.

Optionally the **warning** field can be defined and if it is evaluated to true
then an false **assertion** will **not** generate an error. Code will be generated,
but a warning output will be shown during code generation.

The **assert** block has hierarchical behavior, any assert blocks defined in a
directory will be applied to all stacks inside this directory. For example, an
**assert** block defined on the root of a project will be applied to all stacks
in the project.
