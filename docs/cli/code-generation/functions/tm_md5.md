---
title: tm_md5 - Functions - Configuration Language
description: |-
  The tm_md5 function computes the MD5 hash of a given string and encodes it
  with hexadecimal digits.
---

# `tm_md5` Function

`tm_md5` computes the MD5 hash of a given string and encodes it with
hexadecimal digits.

The given string is first encoded as UTF-8 and then the MD5 algorithm is applied
as defined in [RFC 1321](https://tools.ietf.org/html/rfc1321). The raw hash is
then encoded to lowercase hexadecimal digits before returning.

Before using this function for anything security-sensitive, refer to
[RFC 6151](https://tools.ietf.org/html/rfc6151) for updated security
considerations applying to the MD5 algorithm.

## Examples

```sh
tm_md5("hello world")
5eb63bbbe01eeed093cb22bb8f5acdc3
```

## Related Functions

* [`tm_filemd5`](./tm_filemd5.md) calculates the same hash from
  the contents of a file rather than from a string value.
