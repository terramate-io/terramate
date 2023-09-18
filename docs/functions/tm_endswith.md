---
title: tm_endswith - Functions - Configuration Language
description: |-
  The tm_endswith function takes two values: a string to check and a suffix string. It returns true if the first string ends with that exact suffix.
---

# `tm_endswith` Function

`tm_endswith` takes two values: a string to check and a suffix string. The function returns true if the first string ends with that exact suffix.

```hcl
tm_endswith(string, suffix)
```

## Examples

```
tm_endswith("hello world", "world")
true

tm_endswith("hello world", "hello")
false
```

## Related Functions

- [`tm_startswith`](./tm_startswith.md) takes two values: a string to check and a prefix string. The function returns true if the string begins with that exact prefix.
