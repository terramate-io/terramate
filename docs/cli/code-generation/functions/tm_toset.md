---
title: tm_toset - Functions - Configuration Language
description: The tm_toset function converts a value to a set.
---

# `tm_toset` Function

`tm_toset` converts its argument to a set value.

Explicit type conversions are rarely necessary in Terraform because it will
convert types automatically where required. Use the explicit type conversion
functions only to normalize types returned in module outputs.

Pass a _list_ value to `tm_toset` to convert it to a set, which will remove any
duplicate elements and discard the ordering of the elements.

## Examples

```sh
tm_toset(["a", "b", "c"])
[
  "a",
  "b",
  "c",
]
```

Since Terraform's concept of a set requires all of the elements to be of the
same type, mixed-typed elements will be converted to the most general type:

```sh
tm_toset(["a", "b", 3])
[
  "3",
  "a",
  "b",
]
```

Set collections are unordered and cannot contain duplicate values, so the
ordering of the argument elements is lost and any duplicate values are
coalesced:

```sh
tm_toset(["c", "b", "b"])
[
  "b",
  "c",
]
```
