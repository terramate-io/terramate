---
title: tm_contains - Functions - Configuration Language
description: The tm_contains function determines whether a list or set contains a given value.
---

# `tm_contains` Function

`tm_contains` determines whether a given list or set contains a given single value
as one of its elements.

```hcl
tm_contains(list, value)
```

## Examples

```sh
tm_contains(["a", "b", "c"], "a")
true
tm_contains(["a", "b", "c"], "d")
false
```
