---
title: tm_can - Functions - Configuration Language
description: |-
  The tm_can function tries to evaluate an expression given as an argument and
  indicates whether the evaluation succeeded.
---

# `tm_can` Function

`tm_can` evaluates the given expression and returns a boolean value indicating
whether the expression produced a result without any errors.

This is a special function that is able to catch errors produced when evaluating
its argument. For most situations where you could use `tm_can` it's better to use
[`tm_try`](./tm_try.md) instead, because it allows for more concise definition of
fallback values for failing expressions.

The primary purpose of `tm_can` is to turn an error condition into a boolean
validation result when writing
[custom variable validation rules](https://developer.hashicorp.com/terraform/language/values/variables#custom-validation-rules).
For example:

```hcl
variable "timestamp" {
  type        = string

  validation {
    # formatdate fails if the second argument is not a valid timestamp
    condition     = tm_can(formatdate("", var.timestamp))
    error_message = "The timestamp argument requires a valid RFC 3339 timestamp."
  }
}
```

The `tm_can` function can only catch and handle _dynamic_ errors resulting from
access to data that isn't known until runtime. It will not catch errors
relating to expressions that can be proven to be invalid for any input, such
as a malformed resource reference.

~> **Warning:** The `tm_can` function is intended only for simple tests in
variable validation rules. Although it can technically accept any sort of
expression and be used elsewhere in the configuration, we recommend against
using it in other contexts. For error handling elsewhere in the configuration,
prefer to use [`tm_try`](./tm_try.md).

## Examples

```sh
> local.foo
{
  "bar" = "baz"
}
tm_can(local.foo.bar)
true
tm_can(local.foo.boop)
false
```

The `tm_can` function will _not_ catch errors relating to constructs that are
provably invalid even before dynamic expression evaluation, such as a malformed
reference or a reference to a top-level object that has not been declared:

```sh
tm_can(local.nonexist)

Error: Reference to undeclared local value

A local value with the name "nonexist" has not been declared.
```

## Related Functions

* [`tm_try`](./tm_try.md), which tries evaluating a sequence of expressions and
  returns the result of the first one that succeeds.
