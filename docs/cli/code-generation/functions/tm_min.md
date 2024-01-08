---
title: tm_min - Functions - Configuration Language
description: The tm_min function takes one or more numbers and returns the smallest number.
---

# `tm_min` Function

`tm_min` takes one or more numbers and returns the smallest number from the set.

## Examples

```sh
tm_min(12, 54, 3)
3
```

If the numbers are in a list or set value, use `...` to expand the collection
to individual arguments:

```sh
tm_min([12, 54, 3]...)
3
```

## Related Functions

* [`tm_max`](./tm_max.md), which returns the _greatest_ number from a set.
