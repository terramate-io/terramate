---
title: tm_sha256 - Functions - Configuration Language
description: |-
  The tm_sha256 function computes the SHA256 hash of a given string and encodes it
  with hexadecimal digits.
---

# `tm_sha256` Function

`tm_sha256` computes the SHA256 hash of a given string and encodes it with
hexadecimal digits.

The given string is first encoded as UTF-8 and then the SHA256 algorithm is applied
as defined in [RFC 4634](https://tools.ietf.org/html/rfc4634). The raw hash is
then encoded to lowercase hexadecimal digits before returning.

## Examples

```sh
tm_sha256("hello world")
b94d27b9934d3e08a52e52d7da7dabfac484efe37a5380ee9088f7ace2efcde9
```

## Related Functions

* [`tm_filesha256`](./tm_filesha256.md) calculates the same hash from
  the contents of a file rather than from a string value.
* [`tm_base64sha256`](./tm_base64sha256.md) calculates the same hash but returns
  the result in a more-compact Base64 encoding.
