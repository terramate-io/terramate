---
title: tm_transpose - Functions - Configuration Language
description: |-
  The tm_transpose function takes a map of lists of strings and swaps the keys
  and values.
---

# `tm_transpose` Function

`tm_transpose` takes a map of lists of strings and swaps the keys and values
to produce a new map of lists of strings.

## Examples

```sh
tm_transpose({"a" = ["1", "2"], "b" = ["2", "3"]})
{
  "1" = [
    "a",
  ],
  "2" = [
    "a",
    "b",
  ],
  "3" = [
    "b",
  ],
}
```
