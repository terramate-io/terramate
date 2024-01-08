---
title: tm_distinct - Functions - Configuration Language
description: The tm_distinct function removes duplicate elements from a list.
---

# `tm_distinct` Function

`tm_distinct` takes a list and returns a new list with any duplicate elements
removed.

The first occurrence of each value is retained and the relative ordering of
these elements is preserved.

## Examples

```sh
tm_distinct(["a", "b", "a", "c", "d", "b"])
[
  "a",
  "b",
  "c",
  "d",
]
```
