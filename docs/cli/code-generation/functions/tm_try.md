---
title: tm_try - Functions - Configuration Language
description: |-
  The tm_try function tries to evaluate a sequence of expressions given as
  arguments and returns the result of the first one that does not produce
  any errors.
---

# `tm_try` Function

`tm_try` evaluates all of its argument expressions in turn and returns the result
of the first one that does not produce any errors.

This is a special function that is able to catch errors produced when evaluating
its arguments, which is particularly useful when working with complex data
structures whose shape is not well-known at implementation time.

For example, if some data is retrieved from an external system in JSON or YAML
format and then decoded, the result may have attributes that are not guaranteed
to be set. We can use `tm_try` to produce a normalized data structure which has
a predictable type that can therefore be used more conveniently elsewhere in
the configuration:

```hcl
locals {
  raw_value = yamldecode(file("${path.module}/example.yaml"))
  normalized_value = {
    name   = tostring(tm_try(local.raw_value.name, null))
    groups = tm_try(local.raw_value.groups, [])
  }
}
```

With the above local value expressions, configuration elsewhere in the module
can refer to `local.normalized_value` attributes without the need to repeatedly
check for and handle absent attributes that would otherwise produce errors.

We can also use `tm_try` to deal with situations where a value might be provided
in two different forms, allowing us to normalize to the most general form:

```hcl
variable "example" {
  type = any
}

locals {
  example = tm_try(
    [tostring(var.example)],
    tolist(var.example),
  )
}
```

The above permits `var.example` to be either a list or a single string. If it's
a single string then it'll be normalized to a single-element list containing
that string, again allowing expressions elsewhere in the configuration to just
assume that `local.example` is always a list.

This second example contains two expressions that can both potentially fail.
For example, if `var.example` were set to `{}` then it could be converted to
neither a string nor a list. If `tm_try` exhausts all of the given expressions
without any succeeding, it will return an error describing all of the problems
it encountered.

We strongly suggest using `tm_try` only in special local values whose expressions
perform normalization, so that the error handling is confined to a single
location in the module and the rest of the module can just use straightforward
references to the normalized structure and thus be more readable for future
maintainers.

The `tm_try` function can only catch and handle _dynamic_ errors resulting from
access to data that isn't known until runtime. It will not catch errors
relating to expressions that can be proven to be invalid for any input, such
as a malformed resource reference.

~> **Warning:** The `tm_try` function is intended only for concise testing of the
presence of and types of object attributes. Although it can technically accept
any sort of expression, we recommend using it only with simple attribute
references and type conversion functions as shown in the examples above.
Overuse of `tm_try` to suppress errors will lead to a configuration that is hard
to understand and maintain.

## Examples

```sh
> local.foo
{
  "bar" = "baz"
}
tm_try(local.foo.bar, "fallback")
baz
tm_try(local.foo.boop, "fallback")
fallback
```

The `tm_try` function will _not_ catch errors relating to constructs that are
provably invalid even before dynamic expression evaluation, such as a malformed
reference or a reference to a top-level object that has not been declared:

```sh
tm_try(local.nonexist, "fallback")

Error: Reference to undeclared local value

A local value with the name "nonexist" has not been declared.
```

## Related Functions

* [`tm_can`](./tm_can.md), which tries evaluating an expression and returns a
  boolean value indicating whether it succeeded.
