---
title: tm_keys - Functions - Configuration Language
description: The tm_keys function returns a list of the keys in a given map.
---

# `tm_keys` Function

`tm_keys` takes a map and returns a list containing the keys from that map.

The keys are returned in lexicographical order, ensuring that the result will
be identical as long as the keys in the map don't change.

## Examples

```sh
tm_keys({a=1, c=2, d=3})
[
  "a",
  "c",
  "d",
]
```

## Related Functions

* [`tm_values`](./tm_values.md) returns a list of the _values_ from a map.
