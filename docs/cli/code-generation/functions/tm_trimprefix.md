---
title: tm_trimprefix - Functions - Configuration Language
description: |-
  The tm_trimprefix function removes the specified prefix from the start of a
  given string.
---

# `tm_trimprefix` Function

`tm_trimprefix` removes the specified prefix from the start of the given string. If the string does not start with the prefix, the string is returned unchanged.

## Examples

```sh
tm_trimprefix("helloworld", "hello")
world
```

```sh
tm_trimprefix("helloworld", "cat")
helloworld
```

## Related Functions

* [`tm_trim`](./tm_trim.md) removes characters at the start and end of a string.
* [`tm_trimsuffix`](./tm_trimsuffix.md) removes a word from the end of a string.
* [`tm_trimspace`](./tm_trimspace.md) removes all types of whitespace from
  both the start and the end of a string.
