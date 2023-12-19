---
title: tm_tomap - Functions - Configuration Language
description: The tm_tomap function converts a value to a map.
---

# `tm_tomap` Function

`tm_tomap` converts its argument to a map value.

Explicit type conversions are rarely necessary in Terraform because it will
convert types automatically where required. Use the explicit type conversion
functions only to normalize types returned in module outputs.

## Examples

```sh
tm_tomap({"a" = 1, "b" = 2})
{
  "a" = 1
  "b" = 2
}
```

Since Terraform's concept of a map requires all of the elements to be of the
same type, mixed-typed elements will be converted to the most general type:

```sh
tm_tomap({"a" = "foo", "b" = true})
{
  "a" = "foo"
  "b" = "true"
}
```
