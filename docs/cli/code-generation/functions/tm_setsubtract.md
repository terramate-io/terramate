---
title: tm_setsubtract - Functions - Configuration Language
description: |-
  The tm_setsubtract function returns a new set containing the elements
  from the first set that are not present in the second set
---

# `tm_setsubtract` Function

The `tm_setsubtract` function returns a new set containing the elements from the first set that are not present in the second set. In other words, it computes the
[relative complement](https://en.wikipedia.org/wiki/Complement_\(set_theory\)#Relative_complement) of the second set.

```hcl
tm_setsubtract(a, b)
```

## Examples

```sh
tm_setsubtract(["a", "b", "c"], ["a", "c"])
toset([
  "b",
])
```

### Set Difference (Symmetric Difference)

```sh
tm_setunion(tm_setsubtract(["a", "b", "c"], ["a", "c", "d"]), tm_setsubtract(["a", "c", "d"], ["a", "b", "c"]))
toset([
  "b",
  "d",
])
```

## Related Functions

* [`tm_setintersection`](./tm_setintersection.md) computes the _intersection_ of multiple sets
* [`tm_setproduct`](./tm_setproduct.md) computes the _Cartesian product_ of multiple
  sets.
* [`tm_setunion`](./tm_setunion.md) computes the _union_ of
  multiple sets.
