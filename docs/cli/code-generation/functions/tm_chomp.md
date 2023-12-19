---
title: tm_chomp - Functions - Configuration Language
description: The tm_chomp function removes newline characters at the end of a string.
---

# `tm_chomp` Function

`tm_chomp` removes newline characters at the end of a string.

This can be useful if, for example, the string was read from a file that has
a newline character at the end.

## Examples

```sh
tm_chomp("hello\n")
hello
tm_chomp("hello\r\n")
hello
tm_chomp("hello\n\n")
hello
```

## Related Functions

* [`tm_trimspace`](./tm_trimspace.md), which removes all types of whitespace from
  both the start and the end of a string.
