---
title: tm_compact - Functions - Configuration Language
description: The tm_compact function removes null or empty string elements from a list.
---

# `tm_compact` Function

`tm_compact` takes a list of strings and returns a new list with any null or empty string
elements removed.

## Examples

```sh
tm_compact(["a", "", "b", null, "c"])
[
  "a",
  "b",
  "c",
]
```
