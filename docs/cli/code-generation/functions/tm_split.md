---
title: tm_split - Functions - Configuration Language
description: |-
  The tm_split function produces a list by dividing a given string at all
  occurrences of a given separator.
---

# `tm_split` Function

`tm_split` produces a list by dividing a given string at all occurrences of a
given separator.

```hcl
tm_split(separator, string)
```

## Examples

```sh
tm_split(",", "foo,bar,baz")
[
  "foo",
  "bar",
  "baz",
]
tm_split(",", "foo")
[
  "foo",
]
tm_split(",", "")
[
  "",
]
```

## Related Functions

* [`tm_join`](./tm_join.md) performs the opposite operation: producing a string
  joining together a list of strings with a given separator.
