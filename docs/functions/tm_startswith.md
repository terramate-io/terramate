---
title: tm_startswith - Functions - Configuration Language
description: |-
  The tm_startswith function  takes two values: a string to check and a prefix string. It returns true if the string begins with that exact prefix.
---

# `tm_startswith` Function

`tm_startswith` takes two values: a string to check and a prefix string. The function returns true if the string begins with that exact prefix.

```hcl
tm_startswith(string, prefix)
```

## Examples

```
tm_startswith("hello world", "hello")
true

tm_startswith("hello world", "world")
false
```

## Related Functions

- [`tm_endswith`](./tm_endswith.md) takes two values: a string to check and a suffix string. The function returns true if the first string ends with that exact suffix.
