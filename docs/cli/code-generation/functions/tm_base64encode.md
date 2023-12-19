---
title: tm_base64encode - Functions - Configuration Language
description: The tm_base64encode function applies Base64 encoding to a string.
---

# `tm_base64encode` Function

`tm_base64encode` applies Base64 encoding to a string.

Terraform uses the "standard" Base64 alphabet as defined in
[RFC 4648 section 4](https://tools.ietf.org/html/rfc4648#section-4).

Strings in the Terraform language are sequences of unicode characters rather
than bytes, so this function will first encode the characters from the string
as UTF-8, and then apply Base64 encoding to the result.

The Terraform language applies Unicode normalization to all strings, and so
passing a string through `tm_base64decode` and then `tm_base64encode` may not yield
the original result exactly.

While we do not recommend manipulating large, raw binary data in the Terraform
language, Base64 encoding is the standard way to represent arbitrary byte
sequences, and so resource types that accept or return binary data will use
Base64 themselves, and so this function exists primarily to allow string
data to be easily provided to resource types that expect Base64 bytes.

`tm_base64encode` is, in effect, a shorthand for calling
[`tm_textencodebase64`](./tm_textencodebase64.md) with the encoding name set to
`UTF-8`.

## Examples

```sh
tm_base64encode("Hello World")
SGVsbG8gV29ybGQ=
```

## Related Functions

* [`tm_base64decode`](./tm_base64decode.md) performs the opposite operation,
  decoding Base64 data and interpreting it as a UTF-8 string.
* [`tm_textencodebase64`](./tm_textencodebase64.md) is a more general function that
  supports character encodings other than UTF-8.
* [`tm_base64gzip`](./tm_base64gzip.md) applies gzip compression to a string
  and returns the result with Base64 encoding all in one operation.
* [`tm_filebase64`](./tm_filebase64.md) reads a file from the local filesystem
  and returns its raw bytes with Base64 encoding, without creating an
  intermediate Unicode string.
