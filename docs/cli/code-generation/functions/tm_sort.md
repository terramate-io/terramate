---
title: tm_sort - Functions - Configuration Language
description: |-
  The tm_sort function takes a list of strings and returns a new list with those
  strings sorted lexicographically.
---

# `tm_sort` Function

`tm_sort` takes a list of strings and returns a new list with those strings
sorted lexicographically.

The sort is in terms of Unicode codepoints, with higher codepoints appearing
after lower ones in the result.

## Examples

```sh
tm_sort(["e", "d", "a", "x"])
[
  "a",
  "d",
  "e",
  "x",
]
```
