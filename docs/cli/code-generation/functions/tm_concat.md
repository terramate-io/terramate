---
title: tm_concat - Functions - Configuration Language
description: The tm_concat function combines two or more lists into a single list.
---

# `tm_concat` Function

`tm_concat` takes two or more lists and combines them into a single list.

## Examples

```sh
tm_concat(["a", ""], ["b", "c"])
[
  "a",
  "",
  "b",
  "c",
]
```
