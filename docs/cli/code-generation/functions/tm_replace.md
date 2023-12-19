---
title: tm_replace - Functions - Configuration Language
description: |-
  The tm_replace function searches a given string for another given substring,
  and replaces all occurrences with a given replacement string.
---

# `tm_replace` Function

`tm_replace` searches a given string for another given substring, and replaces
each occurrence with a given replacement string.

```hcl
tm_replace(string, substring, replacement)
```

If `substring` is wrapped in forward slashes, it is treated as a regular
expression, using the same pattern syntax as
[`tm_regex`](./tm_regex.md). If using a regular expression for the substring
argument, the `replacement` string can incorporate captured strings from
the input by using an `$n` sequence, where `n` is the index or name of a
capture group.

## Examples

```sh
tm_replace("1 + 2 + 3", "+", "-")
1 - 2 - 3

tm_replace("hello world", "/w.*d/", "everybody")
hello everybody
```

## Related Functions

- [`tm_regex`](./tm_regex.md) searches a given string for a substring matching a
  given regular expression pattern.
