---
title: tm_jsonencode - Functions - Configuration Language
description: The tm_jsonencode function encodes a given value as a JSON string.
---

# `tm_jsonencode` Function

`tm_jsonencode` encodes a given value to a string using JSON syntax.

The JSON encoding is defined in [RFC 7159](https://tools.ietf.org/html/rfc7159).

This function maps **Terramate language values** to JSON values in the following way:

| Terramate type | JSON type |
| -------------- | --------- |
| `string`       | String    |
| `number`       | Number    |
| `bool`         | Bool      |
| `list(...)`    | Array     |
| `set(...)`     | Array     |
| `tuple(...)`   | Array     |
| `map(...)`     | Object    |
| `object(...)`  | Object    |
| Null value     | `null`    |

Since the JSON format cannot fully represent all of the Terramate language
types, passing the `tm_jsonencode` result to `tm_jsondecode` will not produce an
identical value, but the automatic type conversion rules mean that this is
rarely a problem in practice.

When encoding strings, this function escapes some characters using
Unicode escape sequences: replacing `<`, `>`, `&`, `U+2028`, and `U+2029` with
`\u003c`, `\u003e`, `\u0026`, `\u2028`, and `\u2029`. 

The `jsonencode` command outputs a minified representation of the input.

## Examples

```sh
tm_jsonencode({"hello"="world"})
{"hello":"world"}
```

## Related Functions

* [`tm_jsondecode`](./tm_jsondecode.md) performs the opposite operation, _decoding_
  a JSON string to obtain its represented value.
