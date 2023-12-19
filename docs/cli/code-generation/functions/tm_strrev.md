---
title: tm_strrev - Functions - Configuration Language
description: The tm_strrev function reverses a string.
---

# `tm_strrev` Function

`tm_strrev` reverses the characters in a string.
Note that the characters are treated as _Unicode characters_ (in technical terms, Unicode [grapheme cluster boundaries](https://unicode.org/reports/tr29/#Grapheme_Cluster_Boundaries) are respected).

```hcl
tm_strrev(string)
```

## Examples

```sh
tm_strrev("hello")
olleh
tm_strrev("a ☃")
☃ a
```

## Related Functions

* [`tm_reverse`](./tm_reverse.md) reverses a sequence.
