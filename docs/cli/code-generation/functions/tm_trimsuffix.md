---
title: tm_trimsuffix - Functions - Configuration Language
description: |-
  The tm_trimsuffix function removes the specified suffix from the end of a
  given string.
---

# `tm_trimsuffix` Function

`tm_trimsuffix` removes the specified suffix from the end of the given string.

## Examples

```sh
tm_trimsuffix("helloworld", "world")
hello
```

## Related Functions

* [`tm_trim`](./tm_trim.md) removes characters at the start and end of a string.
* [`tm_trimprefix`](./tm_trimprefix.md) removes a word from the start of a string.
* [`tm_trimspace`](./tm_trimspace.md) removes all types of whitespace from
  both the start and the end of a string.
