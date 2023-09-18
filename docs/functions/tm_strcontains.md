---
title: tm_strcontains - Functions - Configuration Language
description: |-
  The tm_strcontains function checks whether a given string can be found within another string.
---

# `tm_strcontains` Function

`tm_strcontains` function checks whether a substring is within another string.

```hcl
tm_strcontains(string, substr)
```

## Examples

```
tm_strcontains("hello world", "wor")
true
```

```
tm_strcontains("hello world", "wod")
false
```
