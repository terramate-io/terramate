---
title: tm_matchkeys - Functions - Configuration Language
description: |-
  The tm_matchkeys function takes a subset of elements from one list by matching
  corresponding indexes in another list.
---

# `tm_matchkeys` Function

`tm_matchkeys` constructs a new list by taking a subset of elements from one
list whose indexes match the corresponding indexes of values in another
list.

```hcl
tm_matchkeys(valueslist, keyslist, searchset)
```

`tm_matchkeys` identifies the indexes in `keyslist` that are equal to elements of
`searchset`, and then constructs a new list by taking those same indexes from
`valueslist`. Both `valueslist` and `keyslist` must be the same length.

The ordering of the values in `valueslist` is preserved in the result.

## Examples

```sh
tm_matchkeys(["i-123", "i-abc", "i-def"], ["us-west", "us-east", "us-east"], ["us-east"])
[
  "i-abc",
  "i-def",
]
```

If the result ordering is not significant, you can achieve a similar result
using a `for` expression with a map:

```sh
> [for i, z in {"i-123"="us-west","i-abc"="us-east","i-def"="us-east"}: i if z == "us-east"]
[
  "i-def",
  "i-abc",
]
```

If the keys and values of interest are attributes of objects in a list of
objects then you can also achieve a similar result using a `for` expression
with that list:

```sh
> [for x in [{id="i-123",zone="us-west"},{id="i-abc",zone="us-east"}]: x.id if x.zone == "us-east"]
[
  "i-abc",
]
```

For example, the previous form can be used with the list of resource instances
produced by a `resource` block with the `count` meta-attribute set, to filter
the instances by matching one of the resource attributes:

```sh
> [for x in aws_instance.example: x.id if x.availability_zone == "us-east-1a"]
[
  "i-abc123",
  "i-def456",
]
```

Since the signature of `tm_matchkeys` is complicated and not immediately clear to
the reader when used in configuration, prefer to use `for` expressions where
possible to maximize readability.
