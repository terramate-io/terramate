---
title: tm_trimspace - Functions - Configuration Language
description: |-
  The tm_trimspace function removes space characters from the start and end of
  a given string.
---

# `tm_trimspace` Function

`tm_trimspace` removes any space characters from the start and end of the given
string.

This function follows the Unicode definition of "space", which includes
regular spaces, tabs, newline characters, and various other space-like
characters.

## Examples

```sh
tm_trimspace("  hello\n\n")
hello
```

## Related Functions

* [`tm_chomp`](./tm_chomp.md) removes just line ending characters from the _end_ of
  a string.
