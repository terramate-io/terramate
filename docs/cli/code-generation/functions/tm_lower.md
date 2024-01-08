---
title: tm_lower - Functions - Configuration Language
description: >-
  The tm_lower function converts all cased letters in the given string to
  lowercase.
---

# `tm_lower` Function

`tm_lower` converts all cased letters in the given string to lowercase.

## Examples

```sh
tm_lower("HELLO")
hello
tm_lower("АЛЛО!")
алло!
```

This function uses Unicode's definition of letters and of upper- and lowercase.

## Related Functions

* [`tm_upper`](./tm_upper.md) converts letters in a string to _uppercase_.
* [`tm_title`](./tm_title.md) converts the first letter of each word in a string to uppercase.
