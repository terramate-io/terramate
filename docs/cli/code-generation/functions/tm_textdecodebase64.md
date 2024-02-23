---
title: tm_textdecodebase64 - Functions - Configuration Language
description: >-
  The tm_textdecodebase64 function decodes a string that was previously
  Base64-encoded,

  and then interprets the result as characters in a specified character
  encoding.
---

# `tm_textdecodebase64` Function

`tm_textdecodebase64` function decodes a string that was previously Base64-encoded,
and then interprets the result as characters in a specified character encoding.

Terraform uses the "standard" Base64 alphabet as defined in
[RFC 4648 section 4](https://tools.ietf.org/html/rfc4648#section-4).

The `encoding_name` argument must contain one of the encoding names or aliases
recorded in
[the IANA character encoding registry](https://www.iana.org/assignments/character-sets/character-sets.xhtml).
Terraform supports only a subset of the registered encodings, and the encoding
support may vary between Terraform versions.

Terraform accepts the encoding name `UTF-8`, which will produce the same result
as [`tm_base64decode`](./tm_base64decode.md).

## Examples

```sh
tm_textdecodebase64("SABlAGwAbABvACAAVwBvAHIAbABkAA==", "UTF-16LE")
Hello World
```

## Related Functions

* [`tm_textencodebase64`](./tm_textencodebase64.md) performs the opposite operation,
  applying target encoding and then Base64 to a string.
* [`tm_base64decode`](./tm_base64decode.md) is effectively a shorthand for
  `tm_textdecodebase64` where the character encoding is fixed as `UTF-8`.
