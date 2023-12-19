---
title: tm_merge - Functions - Configuration Language
description: |-
  The tm_merge function takes an arbitrary number maps or objects, and returns a
  single map or object that contains a merged set of elements from all
  arguments.
---

# `tm_merge` Function

`tm_merge` takes an arbitrary number of maps or objects, and returns a single map
or object that contains a merged set of elements from all arguments.

If more than one given map or object defines the same key or attribute, then
the one that is later in the argument sequence takes precedence. If the
argument types do not match, the resulting type will be an object matching the
type structure of the attributes after the merging rules have been applied.

## Examples

```sh
tm_merge({a="b", c="d"}, {e="f", c="z"})
{
  "a" = "b"
  "c" = "z"
  "e" = "f"
}
```

```sh
tm_merge({a="b"}, {a=[1,2], c="z"}, {d=3})
{
  "a" = [
    1,
    2,
  ]
  "c" = "z"
  "d" = 3
}
```

The following example uses the expansion symbol (...) to transform the value into separate arguments. Refer to [Expanding Function Argument](https://developer.hashicorp.com/terraform/language/expressions/function-calls#expanding-function-arguments) for details.

```sh
tm_merge([{a="b", c="d"}, {}, {e="f", c="z"}]...)
{
  "a" = "b"
  "c" = "z"
  "e" = "f"
}
```
