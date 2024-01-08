---
title: tm_anytrue - Functions - Configuration Language
description: |-
  The tm_anytrue function determines whether any element of a collection
  is true or "true". If the collection is empty, it returns false.
---

# `tm_anytrue` Function

`tm_anytrue` returns `true` if any element in a given collection is `true`
or `"true"`. It also returns `false` if the collection is empty.

```hcl
tm_anytrue(list)
```

## Examples

```sh
tm_anytrue(["true"])
true
tm_anytrue([true])
true
tm_anytrue([true, false])
true
tm_anytrue([])
false
```
