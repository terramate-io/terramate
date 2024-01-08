---
title: tm_log - Functions - Configuration Language
description: The tm_log function returns the logarithm of a given number in a given base.
---

# `tm_log` Function

`tm_log` returns the logarithm of a given number in a given base.

```hcl
tm_log(number, base)
```

## Examples

```sh
tm_log(50, 10)
1.6989700043360185
tm_log(16, 2)
4
```

`log` and `ceil` can be used together to find the minimum number of binary
digits required to represent a given number of distinct values:

```sh
tm_ceil(log(15, 2))
4
tm_ceil(log(16, 2))
4
tm_ceil(log(17, 2))
5
```
