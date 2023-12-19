---
title: tm_base64sha512 - Functions - Configuration Language
description: |-
  The tm_base64sha512 function computes the SHA512 hash of a given string and
  encodes it with Base64.
---

# `tm_base64sha512` Function

`tm_base64sha512` computes the SHA512 hash of a given string and encodes it with
Base64. This is not equivalent to `tm_base64encode(tm_sha512("test"))` since 
`tm_sha512()` returns hexadecimal representation.

The given string is first encoded as UTF-8 and then the SHA512 algorithm is applied
as defined in [RFC 4634](https://tools.ietf.org/html/rfc4634). The raw hash is
then encoded with Base64 before returning. Terraform uses the "standard" Base64
alphabet as defined in [RFC 4648 section 4](https://tools.ietf.org/html/rfc4648#section-4).

## Examples

```sh
tm_base64sha512("hello world")
MJ7MSJwS1utMxA9QyQLytNDtd+5RGnx6m808qG1M2G+YndNbxf9JlnDaNCVbRbDP2DDoH2Bdz33FVC6TrpzXbw==
```

## Related Functions

* [`tm_filebase64sha512`](./tm_filebase64sha512.md) calculates the same hash from
  the contents of a file rather than from a string value.
* [`tm_sha512`](./tm_sha512.md) calculates the same hash but returns the result
  in a more-verbose hexadecimal encoding.
