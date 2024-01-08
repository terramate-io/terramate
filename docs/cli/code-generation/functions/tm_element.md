---
title: tm_element - Functions - Configuration Language
description: The tm_element function retrieves a single element from a list.
---

# `tm_element` Function

`tm_element` retrieves a single element from a list.

```hcl
tm_element(list, index)
```

The index is zero-based. This function produces an error if used with an
empty list. The index must be a non-negative integer.

Use the built-in index syntax `list[index]` in most cases. Use this function
only for the special additional "wrap-around" behavior described below.

## Examples

```sh
tm_element(["a", "b", "c"], 1)
b
```

If the given index is greater than the length of the list then the index is
"wrapped around" by taking the index modulo the length of the list:

```sh
tm_element(["a", "b", "c"], 3)
a
```

To get the last element from the list use [`tm_length`](./tm_length.md) to find
the size of the list (minus 1 as the list is zero-based) and then pick the
last element:

```sh
tm_element(["a", "b", "c"], length(["a", "b", "c"])-1)
c
```

## Related Functions

* [`tm_index`](./tm_index.md) finds the index for a particular element value.
* [`tm_lookup`](./tm_lookup.md) retrieves a value from a _map_ given its _key_.
