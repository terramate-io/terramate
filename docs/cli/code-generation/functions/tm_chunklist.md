---
title: tm_chunklist - Functions - Configuration Language
description: |-
  The tm_chunklist function splits a single list into fixed-size chunks, returning
  a list of lists.
---

# `tm_chunklist` Function

`chunklist` splits a single list into fixed-size chunks, returning a list of lists.

```hcl
tm_chunklist(list, chunk_size)
```

## Examples

```sh
tm_chunklist(["a", "b", "c", "d", "e"], 2)
[
  [
    "a",
    "b",
  ],
  [
    "c",
    "d",
  ],
  [
    "e",
  ],
]
tm_chunklist(["a", "b", "c", "d", "e"], 1)
[
  [
    "a",
  ],
  [
    "b",
  ],
  [
    "c",
  ],
  [
    "d",
  ],
  [
    "e",
  ],
]
```
