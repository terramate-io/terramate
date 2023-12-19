---
title: tm_upper - Functions - Configuration Language
description: >-
  The tm_upper function converts all cased letters in the given string to
  uppercase.
---

# `tm_upper` Function

`tm_upper` converts all cased letters in the given string to uppercase.

## Examples

```sh
tm_upper("hello")
HELLO
tm_upper("алло!")
АЛЛО!
```

This function uses Unicode's definition of letters and of upper- and lowercase.

## Related Functions

* [`tm_lower`](./tm_lower.md) converts letters in a string to _lowercase_.
* [`tm_title`](./tm_title.md) converts the first letter of each word in a string to uppercase.
