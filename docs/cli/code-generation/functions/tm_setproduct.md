---
title: tm_setproduct - Functions - Configuration Language
description: |-
  The tm_setproduct function finds all of the possible combinations of elements
  from all of the given sets by computing the cartesian product.
---

# `tm_setproduct` Function

The `tm_setproduct` function finds all of the possible combinations of elements
from all of the given sets by computing the
[Cartesian product](https://en.wikipedia.org/wiki/Cartesian_product).

```hcl
tm_setproduct(sets...)
```

This function is particularly useful for finding the exhaustive set of all
combinations of members of multiple sets, such as per-application-per-environment
resources.

```sh
tm_setproduct(["development", "staging", "production"], ["app1", "app2"])
[
  [
    "development",
    "app1",
  ],
  [
    "development",
    "app2",
  ],
  [
    "staging",
    "app1",
  ],
  [
    "staging",
    "app2",
  ],
  [
    "production",
    "app1",
  ],
  [
    "production",
    "app2",
  ],
]
```

You must pass at least two arguments to this function.

Although defined primarily for sets, this function can also work with lists.
If all of the given arguments are lists then the result is a list, preserving
the ordering of the given lists. Otherwise the result is a set. In either case,
the result's element type is a list of values corresponding to each given
argument in turn.

## Examples

There is an example of the common usage of this function above. There are some
other situations that are less common when hand-writing but may arise in
reusable module situations.

If any of the arguments is empty then the result is always empty itself,
similar to how multiplying any number by zero gives zero:

```sh
tm_setproduct(["development", "staging", "production"], [])
[]
```

Similarly, if all of the arguments have only one element then the result has
only one element, which is the first element of each argument:

```sh
tm_setproduct(["a"], ["b"])
[
  [
    "a",
    "b",
  ],
]
```

Each argument must have a consistent type for all of its elements. If not,
Terramate will attempt to convert to the most general type, or produce an
error if such a conversion is impossible. For example, mixing both strings and
numbers results in the numbers being converted to strings so that the result
elements all have a consistent type:

```sh
tm_setproduct(["staging", "production"], ["a", 2])
[
  [
    "staging",
    "a",
  ],
  [
    "staging",
    "2",
  ],
  [
    "production",
    "a",
  ],
  [
    "production",
    "2",
  ],
]
```

## Related Functions

- [`tm_contains`](./tm_contains.md) tests whether a given list or set contains
  a given element value.
- [`tm_flatten`](./tm_flatten.md) is useful for flattening hierarchical data
  into a single list, for situations where the relationships between two
  object types are defined explicitly.
- [`tm_setintersection`](./tm_setintersection.md) computes the _intersection_ of
  multiple sets.
- [`tm_setsubtract`](./tm_setsubtract.md) computes the _relative complement_ of two sets
- [`tm_setunion`](./tm_setunion.md) computes the _union_ of multiple
  sets.
