---
title: tm_textencodebase64 - Functions - Configuration Language
description: >-
  The tm_textencodebase64 function encodes the unicode characters in a given string
  using a specified character encoding, returning the result base64 encoded.
---

# `tm_textencodebase64` Function

`tm_textencodebase64` encodes the unicode characters in a given string using a
specified character encoding, returning the result base64 encoded because
Terraform language strings are always sequences of unicode characters.

```hcl
tm_substr(string, encoding_name)
```

Terraform uses the "standard" Base64 alphabet as defined in
[RFC 4648 section 4](https://tools.ietf.org/html/rfc4648#section-4).

The `encoding_name` argument must contain one of the encoding names or aliases
recorded in
[the IANA character encoding registry](https://www.iana.org/assignments/character-sets/character-sets.xhtml).
Terraform supports only a subset of the registered encodings, and the encoding
support may vary between Terraform versions. In particular Terraform supports
`UTF-16LE`, which is the native character encoding for the Windows API and
therefore sometimes expected by Windows-originated software such as PowerShell.

Terraform also accepts the encoding name `UTF-8`, which will produce the same
result as [`tm_base64encode`](./tm_base64encode.md).

## Examples

```sh
tm_textencodebase64("Hello World", "UTF-16LE")
SABlAGwAbABvACAAVwBvAHIAbABkAA==
```

## Related Functions

* [`tm_textdecodebase64`](./tm_textdecodebase64.md) performs the opposite operation,
  decoding Base64 data and interpreting it as a particular character encoding.
* [`tm_base64encode`](./tm_base64encode.md) applies Base64 encoding of the UTF-8
  encoding of a string.
* [`tm_filebase64`](./tm_filebase64.md) reads a file from the local filesystem
  and returns its raw bytes with Base64 encoding, without creating an
  intermediate Unicode string.
