---
title: tm_join - Functions - Configuration Language
description: |-
  The tm_join function produces a string by concatenating the elements of a list
  with a given delimiter.
---

# `tm_join` Function

`tm_join` produces a string by concatenating all of the elements of the specified
list of strings with the specified separator.

```hcl
tm_join(separator, list)
```

## Examples

```sh
tm_join("-", ["foo", "bar", "baz"])
"foo-bar-baz"
tm_join(", ", ["foo", "bar", "baz"])
foo, bar, baz
tm_join(", ", ["foo"])
foo
```

## Related Functions

* [`tm_split`](./tm_split.md) performs the opposite operation: producing a list
  by separating a single string using a given delimiter.
