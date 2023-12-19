---
title: tm_alltrue - Functions - Configuration Language
description: |-
  The tm_alltrue function determines whether all elements of a collection
  are true or "true". If the collection is empty, it returns true.
---

# `tm_alltrue` Function

`tm_alltrue` returns `true` if all elements in a given collection are `true`
or `"true"`. It also returns `true` if the collection is empty.

```hcl
tm_alltrue(list)
```

## Examples

```sh
tm_alltrue(["true", true])
true
tm_alltrue([true, false])
false
```
