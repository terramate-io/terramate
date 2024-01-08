---
title: tm_setintersection - Functions - Configuration Language
description: |-
  The tm_setintersection function takes multiple sets and produces a single set
  containing only the elements that all of the given sets have in common.
---

# `tm_setintersection` Function

The `tm_setintersection` function takes multiple sets and produces a single set
containing only the elements that all of the given sets have in common.
In other words, it computes the
[intersection](https://en.wikipedia.org/wiki/Intersection_\(set_theory\)) of the sets.

```hcl
tm_setintersection(sets...)
```

## Examples

```sh
tm_setintersection(["a", "b"], ["b", "c"], ["b", "d"])
[
  "b",
]
```

The given arguments are converted to sets, so the result is also a set and
the ordering of the given elements is not preserved.

## Related Functions

* [`tm_contains`](./tm_contains.md) tests whether a given list or set contains
  a given element value.
* [`tm_setproduct`](./tm_setproduct.md) computes the _Cartesian product_ of multiple
  sets.
* [`tm_setsubtract`](./tm_setsubtract.md) computes the _relative complement_ of two sets
* [`tm_setunion`](./tm_setunion.md) computes the _union_ of
  multiple sets.
