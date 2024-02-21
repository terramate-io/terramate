---
title: tm_regexall - Functions - Configuration Language
description: >-
  The tm_regex function applies a regular expression to a string and returns a list
  of all matches.
---

# `tm_regexall` Function

`tm_regexall` applies a
[regular expression](https://en.wikipedia.org/wiki/Regular_expression)
to a string and returns a list of all matches.

```hcl
tm_regexall(pattern, string)
```

`tm_regexall` is a variant of [`tm_regex`](./tm_regex.md) and uses the same pattern
syntax. For any given input to `tm_regex`, `tm_regexall` returns a list of whatever
type `tm_regex` would've returned, with one element per match. That is:

- If the pattern has no capture groups at all, the result is a list of
  strings.
- If the pattern has one or more _unnamed_ capture groups, the result is a
  list of lists.
- If the pattern has one or more _named_ capture groups, the result is a
  list of maps.

`tm_regexall` can also be used to test whether a particular string matches a
given pattern, by testing whether the length of the resulting list of matches
is greater than zero.

## Examples

```sh
tm_regexall("[a-z]+", "1234abcd5678efgh9")
[
  "abcd",
  "efgh",
]

tm_length(regexall("[a-z]+", "1234abcd5678efgh9"))
2

tm_length(regexall("[a-z]+", "123456789")) > 0
false
```

## Related Functions

- [`tm_regex`](./tm_regex.md) searches for a single match of a given pattern, and
  returns an error if no match is found.

If Terramate already has a more specialized function to parse the syntax you
are trying to match, prefer to use that function instead. Regular expressions
can be hard to read and can obscure your intent, making a configuration harder
to read and understand.
