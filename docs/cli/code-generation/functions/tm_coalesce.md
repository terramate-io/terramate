---
title: tm_coalesce - Functions - Configuration Language
description: |-
  The tm_coalesce function takes any number of arguments and returns the
  first one that isn't null nor empty.
---

# `tm_coalesce` Function

`tm_coalesce` takes any number of arguments and returns the first one
that isn't null or an empty string.

All of the arguments must be of the same type. Terraform will try to
convert mismatched arguments to the most general of the types that all
arguments can convert to, or return an error if the types are incompatible.
The result type is the same as the type of all of the arguments.

## Examples

```sh
tm_coalesce("a", "b")
a
tm_coalesce("", "b")
b
tm_coalesce(1,2)
1
```

To perform the `coalesce` operation with a list of strings, use the `...`
symbol to expand the list as arguments:

```sh
tm_coalesce(["", "b"]...)
b
```

Terraform attempts to select a result type that all of the arguments can
convert to, so mixing argument types may produce surprising results due to
Terraform's automatic type conversion rules:

```sh
tm_coalesce(1, "hello")
"1"
tm_coalesce(true, "hello")
"true"
tm_coalesce({}, "hello")

Error: Error in function call

Call to function "tm_coalesce" failed: all arguments must have the same type.
```

## Related Functions

* [`tm_coalescelist`](./tm_coalescelist.md) performs a similar operation with
  list arguments rather than individual arguments.
