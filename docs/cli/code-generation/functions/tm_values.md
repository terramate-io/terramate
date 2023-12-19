---
title: tm_values - Functions - Configuration Language
description: The tm_values function returns a list of the element values in a given map.
---

# `tm_values` Function

`tm_values` takes a map and returns a list containing the values of the elements
in that map.

The values are returned in lexicographical order by their corresponding _keys_,
so the values will be returned in the same order as their keys would be
returned from [`tm_keys`](./tm_keys.md).

## Examples

```sh
tm_values({a=3, c=2, d=1})
[
  3,
  2,
  1,
]
```

## Related Functions

* [`tm_keys`](./tm_keys.md) returns a list of the _keys_ from a map.
