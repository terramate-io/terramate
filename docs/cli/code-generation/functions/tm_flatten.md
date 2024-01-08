---
title: tm_flatten - Functions - Configuration Language
description: The tm_flatten function eliminates nested lists from a list.
---

# `tm_flatten` Function

`tm_flatten` takes a list and replaces any elements that are lists with a
flattened sequence of the list contents.

## Examples

```sh
tm_flatten([["a", "b"], [], ["c"]])
["a", "b", "c"]
```

If any of the nested lists also contain directly-nested lists, these too are
flattened recursively:

```sh
tm_flatten([[["a", "b"], []], ["c"]])
["a", "b", "c"]
```

Indirectly-nested lists, such as those in maps, are _not_ flattened.

## Related Functions

* [`tm_setproduct`](./tm_setproduct.md) finds all of the combinations of multiple
  lists or sets of values, which can also be useful when preparing collections
  for use with `for_each` constructs.
