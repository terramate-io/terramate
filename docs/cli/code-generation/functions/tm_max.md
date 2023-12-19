---
title: tm_max - Functions - Configuration Language
description: The tm_max function takes one or more numbers and returns the greatest number.
---

# `tm_max` Function

`tm_max` takes one or more numbers and returns the greatest number from the set.

## Examples

```sh
tm_max(12, 54, 3)
54
```

If the numbers are in a list or set value, use `...` to expand the collection
to individual arguments:

```sh
tm_max([12, 54, 3]...)
54
```

## Related Functions

* [`tm_min`](./tm_min.md), which returns the _smallest_ number from a set.
