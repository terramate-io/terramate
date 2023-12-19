---
title: tm_title - Functions - Configuration Language
description: |-
  The tm_title function converts the first letter of each word in a given string
  to uppercase.
---

# `tm_title` Function

`tm_title` converts the first letter of each word in the given string to uppercase.

## Examples

```sh
tm_title("hello world")
Hello World
```

This function uses Unicode's definition of letters and of upper- and lowercase.

## Related Functions

* [`tm_upper`](./tm_upper.md) converts _all_ letters in a string to uppercase.
* [`tm_lower`](./tm_lower.md) converts all letters in a string to lowercase.
