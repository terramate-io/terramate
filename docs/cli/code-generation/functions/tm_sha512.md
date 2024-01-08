---
title: tm_sha512 - Functions - Configuration Language
description: |-
  The tm_sha512 function computes the SHA512 hash of a given string and encodes it
  with hexadecimal digits.
---

# `tm_sha512` Function

`tm_sha512` computes the SHA512 hash of a given string and encodes it with
hexadecimal digits.

The given string is first encoded as UTF-8 and then the SHA512 algorithm is applied
as defined in [RFC 4634](https://tools.ietf.org/html/rfc4634). The raw hash is
then encoded to lowercase hexadecimal digits before returning.

## Examples

```sh
tm_sha512("hello world")
309ecc489c12d6eb4cc40f50c902f2b4d0ed77ee511a7c7a9bcd3ca86d4cd86f989dd35bc5ff499670da34255b45b0cfd830e81f605dcf7dc5542e93ae9cd76f
```

## Related Functions

* [`tm_filesha512`](./tm_filesha512.md) calculates the same hash from
  the contents of a file rather than from a string value.
* [`tm_base64sha512`](./tm_base64sha512.md) calculates the same hash but returns
  the result in a more-compact Base64 encoding.
