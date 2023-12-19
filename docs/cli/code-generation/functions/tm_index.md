---
title: tm_index - Functions - Configuration Language
description: The tm_index function finds the element index for a given value in a list.
---

# `tm_index` Function

`tm_index` finds the element index for a given value in a list.

```hcl
tm_index(list, value)
```

The tm_returned index is zero-based. This function produces an error if the given
value is not present in the list.

## Examples

```sh
tm_index(["a", "b", "c"], "b")
1
```

## Related Functions

* [`tm_element`](./tm_element.md) retrieves a particular element from a list given
  its index.
