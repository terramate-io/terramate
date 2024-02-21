---
title: tm_indent - Functions - Configuration Language
description: |-
  The tm_indent function adds a number of spaces to the beginnings of all but the
  first line of a given multi-line string.
---

# `tm_indent` Function

`tm_indent` adds a given number of spaces to the beginnings of all but the first
line in a given multi-line string.

```hcl
tm_indent(num_spaces, string)
```

## Examples

This function is useful for inserting a multi-line string into an
already-indented context in another string:

```sh
tm_indent(2, "  items: ${"[\n  foo,\n  bar,\n]\n"}")
  items: [
    foo,
    bar,
  ]
```

The first line of the string is not indented so that, as above, it can be
placed after an introduction sequence that has already begun the line.
