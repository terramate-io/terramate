---
title: tm_lookup - Functions - Configuration Language
description: The tm_lookup function retrieves an element value from a map given its key.
---

# `tm_lookup` Function

`lookup` retrieves the value of a single element from a map, given its key.
If the given key does not exist, the given default value is returned instead.

```
lookup(map, key, default)
```

-> For historical reasons, the `default` parameter is actually optional. However,
omitting `default` is deprecated since v0.7 because that would then be
equivalent to the native index syntax, `map[key]`.

## Examples

```
tm_lookup({a="ay", b="bee"}, "a", "what?")
ay
tm_lookup({a="ay", b="bee"}, "c", "what?")
what?
```

## Related Functions

* [`tm_element`](./tm_element.md) retrieves a value from a _list_ given its _index_.
