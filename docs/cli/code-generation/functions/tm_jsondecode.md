---
title: tm_jsondecode - Functions - Configuration Language
description: |-
  The tm_jsondecode function decodes a JSON string into a representation of its
  value.
---

# `tm_jsondecode` Function

`tm_jsondecode` interprets a given string as JSON, returning a representation
of the result of decoding that string.

The JSON encoding is defined in [RFC 7159](https://tools.ietf.org/html/rfc7159).

This function maps JSON values to
**Terramate language values** in the following way:

| JSON type | Terramate type                                               |
| --------- | ------------------------------------------------------------ |
| String    | `string`                                                     |
| Number    | `number`                                                     |
| Boolean   | `bool`                                                       |
| Object    | `object(...)` with attribute types determined per this table |
| Array     | `tuple(...)` with element types determined per this table    |
| Null      | The Terramate `null` value                          |

The Terramate language automatic type conversion rules mean that you don't
usually need to worry about exactly what type is produced for a given value,
and can just use the result in an intuitive way.

## Examples

```sh
tm_jsondecode("{\"hello\": \"world\"}")
{
  "hello" = "world"
}
tm_jsondecode("true")
true
```

## Related Functions

* [`tm_jsonencode`](./tm_jsonencode.md) performs the opposite operation, _encoding_
  a value as JSON.
