---
title: tm_trim - Functions - Configuration Language
description: >-
  The tm_trim function removes the specified set of characters from the start and
  end of

  a given string.
---

# `tm_trim` Function

`tm_trim` removes the specified set of characters from the start and end of the given
string.

```hcl
tm_trim(string, str_character_set)
```

Every occurrence of a character in the second argument is removed from the start
and end of the string specified in the first argument.

## Examples

```sh
tm_trim("?!hello?!", "!?")
"hello"

tm_trim("foobar", "far")
"oob"

tm_trim("   hello! world.!  ", "! ")
"hello! world."
```

## Related Functions

* [`tm_trimprefix`](./tm_trimprefix.md) removes a word from the start of a string.
* [`tm_trimsuffix`](./tm_trimsuffix.md) removes a word from the end of a string.
* [`tm_trimspace`](./tm_trimspace.md) removes all types of whitespace from
  both the start and the end of a string.
