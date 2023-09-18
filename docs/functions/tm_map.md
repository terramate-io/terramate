---
title: tm_map - Functions - Configuration Language
description: The tm_map function constructs a map from some given elements.
---

# `tm_map` Function

The `tm_map` function is no longer available. Prior to Terraform v0.12 it was
the only available syntax for writing a literal map inside an expression,
but Terraform v0.12 introduced a new first-class syntax.

To update an expression like `map("a", "b", "c", "d")`, write the following instead:

```
tomap({
  a = "b"
  c = "d"
})
```

The `tm_{ ... }` braces construct an object value, and then the `tomap` function
then converts it to a map. For more information on the value types in the
Terraform language, see [Type Constraints](https://developer.hashicorp.com/terraform/language/expressions/types).

## Related Functions

* [`tm_tomap`](./tm_tomap.md) converts an object value to a map.
* [`tm_zipmap`](./tm_zipmap.md) constructs a map dynamically, by taking keys from
  one list and values from another list.
