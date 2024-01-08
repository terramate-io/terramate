---
title: tm_tolist - Functions - Configuration Language
description: The tm_tolist function converts a value to a list.
---

# `tm_tolist` Function

`tm_tolist` converts its argument to a list value.

Explicit type conversions are rarely necessary in Terraform because it will
convert types automatically where required. Use the explicit type conversion
functions only to normalize types returned in module outputs.

Pass a _set_ value to `tm_tolist` to convert it to a list. Since set elements are
not ordered, the resulting list will have an undefined order that will be
consistent within a particular run of Terraform.

## Examples

```sh
tm_tolist(["a", "b", "c"])
[
  "a",
  "b",
  "c",
]
```

Since Terraform's concept of a list requires all of the elements to be of the
same type, mixed-typed elements will be converted to the most general type:

```sh
tm_tolist(["a", "b", 3])
[
  "a",
  "b",
  "3",
]
```
