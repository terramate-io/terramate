---
title: tm_yamlencode - Functions - Configuration Language
description: The tm_yamlencode function encodes a given value as a YAML string.
---

# `tm_yamlencode` Function

`tm_yamlencode` encodes a given value to a string using
[YAML 1.2](https://yaml.org/spec/1.2/spec.html) block syntax.

This function maps
[Terraform language values](https://developer.hashicorp.com/terraform/language/expressions/types)
to YAML tags in the following way:

| Terraform type | YAML type            |
| -------------- | -------------------- |
| `string`       | `!!str`              |
| `number`       | `!!float` or `!!int` |
| `bool`         | `!!bool`             |
| `list(...)`    | `!!seq`              |
| `set(...)`     | `!!seq`              |
| `tuple(...)`   | `!!seq`              |
| `map(...)`     | `!!map`              |
| `object(...)`  | `!!map`              |
| Null value     | `!!null`             |

`tm_yamlencode` uses the implied syntaxes for all of the above types, so it does
not generate explicit YAML tags.

Because the YAML format cannot fully represent all of the Terramate language
types, passing the `tm_yamlencode` result to `tm_yamldecode` will not produce an
identical value, but the Terraform language automatic type conversion rules
mean that this is rarely a problem in practice.

YAML is a superset of JSON, and so where possible we recommend generating
JSON using [`tm_jsonencode`](./tm_jsonencode.md) instead, even if
a remote system supports YAML. JSON syntax is equivalent to flow-style YAML
and Terraform can present detailed structural change information for JSON
values in plans, whereas Terraform will treat block-style YAML just as a normal
multi-line string. However, generating YAML may improve readability if the
resulting value will be directly read or modified in the remote system by
humans.

## Examples

```sh
tm_yamlencode({"a":"b", "c":"d"})
"a": "b"
"c": "d"

tm_yamlencode({"foo":[1, 2, 3], "bar": "baz"})
"bar": "baz"
"foo":
- 1
- 2
- 3

tm_yamlencode({"foo":[1, {"a":"b","c":"d"}, 3], "bar": "baz"})
"bar": "baz"
"foo":
- 1
- "a": "b"
  "c": "d"
- 3
```

`tm_yamlencode` always uses YAML's "block style" for mappings and sequences, unless
the mapping or sequence is empty. To generate flow-style YAML, use
[`tm_jsonencode`](./tm_jsonencode.md) instead: YAML flow-style is a superset
of JSON syntax.

## Related Functions

- [`tm_jsonencode`](./tm_jsonencode.md) is a similar operation using JSON instead
  of YAML.
- [`tm_yamldecode`](./tm_yamldecode.md) performs the opposite operation, _decoding_
  a YAML string to obtain its represented value.
