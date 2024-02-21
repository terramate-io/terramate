---
title: tm_parseint - Functions - Configuration Language
description: >-
  The tm_parseint function parses the given string as a representation of an
  integer.
---

# `tm_parseint` Function

`tm_parseint` parses the given string as a representation of an integer in
the specified base and returns the resulting number. The base must be between 2
and 62 inclusive.

All bases use the arabic numerals 0 through 9 first. Bases between 11 and 36
inclusive use case-insensitive latin letters to represent higher unit values.
Bases 37 and higher use lowercase latin letters and then uppercase latin
letters.

If the given string contains any non-digit characters or digit characters that
are too large for the given base then `tm_parseint` will produce an error.

## Examples

```sh
tm_parseint("100", 10)
100

tm_parseint("FF", 16)
255

tm_parseint("-10", 16)
-16

tm_parseint("1011111011101111", 2)
48879

tm_parseint("aA", 62)
656

tm_parseint("12", 2)

Error: Invalid function argument

Invalid value for "number" parameter: cannot parse "12" as a base 2 integer.
```

## Related Functions

* [`tm_format`](./tm_format.md) can format numbers and other values into strings,
  with optional zero padding, alignment, etc.
