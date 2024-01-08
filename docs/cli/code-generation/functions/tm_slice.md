---
title: tm_slice - Functions - Configuration Language
description: The tm_slice function extracts some consecutive elements from within a list.
---

# `tm_slice` Function

`tm_slice` extracts some consecutive elements from within a list.

```hcl
tm_slice(list, startindex, endindex)
```

`startindex` is inclusive, while `endindex` is exclusive. This function returns
an error if either index is outside the bounds of valid indices for the given
list.

## Examples

```sh
tm_slice(["a", "b", "c", "d"], 1, 3)
[
  "b",
  "c",
]
```

## Related Functions

* [`tm_substr`](./tm_substr.md) performs a similar function for characters in a
  string, although it uses a length instead of an end index.
