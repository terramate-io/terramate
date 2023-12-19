---
title: tm_zipmap - Functions - Configuration Language
description: |-
  The tm_zipmap function constructs a map from a list of keys and a corresponding
  list of values.
---

# `tm_zipmap` Function

`tm_zipmap` constructs a map from a list of keys and a corresponding list of values.

```hcl
tm_zipmap(keyslist, valueslist)
```

Both `keyslist` and `valueslist` must be of the same length. `keyslist` must
be a list of strings, while `valueslist` can be a list of any type.

Each pair of elements with the same index from the two lists will be used
as the key and value of an element in the resulting map. If the same value
appears multiple times in `keyslist` then the value with the highest index
is used in the resulting map.

## Examples

```sh
tm_zipmap(["a", "b"], [1, 2])
{
  "a" = 1
  "b" = 2
}
```
