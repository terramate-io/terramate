---
title: tm_coalescelist - Functions - Configuration Language
description: |-
  The tm_coalescelist function takes any number of list arguments and returns the
  first one that isn't empty.
---

# `tm_coalescelist` Function

`tm_coalescelist` takes any number of list arguments and returns the first one
that isn't empty.

## Examples

```sh
tm_coalescelist(["a", "b"], ["c", "d"])
[
  "a",
  "b",
]
tm_coalescelist([], ["c", "d"])
[
  "c",
  "d",
]
```

To perform the `tm_coalescelist` operation with a list of lists, use the `...`
symbol to expand the outer list as arguments:

```sh
tm_coalescelist([[], ["c", "d"]]...)
[
  "c",
  "d",
]
```

## Related Functions

* [`tm_coalesce`](./tm_coalesce.md) performs a similar operation with string
  arguments rather than list arguments.
