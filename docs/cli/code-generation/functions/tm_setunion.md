---
title: tm_setunion - Functions - Configuration Language
description: |-
  The tm_setunion function takes multiple sets and produces a single set
  containing the elements from all of the given sets.
---

# `tm_setunion` Function

The `tm_setunion` function takes multiple sets and produces a single set
containing the elements from all of the given sets. In other words, it
computes the [union](https://en.wikipedia.org/wiki/Union_\(set_theory\)) of
the sets.

```hcl
tm_setunion(sets...)
```

## Examples

```sh
tm_setunion(["a", "b"], ["b", "c"], ["d"])
[
  "d",
  "b",
  "c",
  "a",
]
```

The given arguments are converted to sets, so the result is also a set and
the ordering of the given elements is not preserved.

## Related Functions

* [`tm_contains`](./tm_contains.md) tests whether a given list or set contains
  a given element value.
* [`tm_setintersection`](./tm_setintersection.md) computes the _intersection_ of
  multiple sets.
* [`tm_setproduct`](./tm_setproduct.md) computes the _Cartesian product_ of multiple
  sets.
* [`tm_setsubtract`](./tm_setsubtract.md) computes the _relative complement_ of two sets
