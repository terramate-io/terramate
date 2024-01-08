---
title: tm_abs - Functions - Configuration Language
description: The tm_abs function returns the absolute value of the given number.
---

# `tm_abs` Function

`tm_abs` returns the absolute value of the given number. In other words, if the
number is zero or positive then it is returned as-is, but if it is negative
then it is multiplied by -1 to make it positive before returning it.

## Examples

```sh
tm_abs(23)
23
tm_abs(0)
0
tm_abs(-12.4)
12.4
```
