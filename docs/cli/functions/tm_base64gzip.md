---
title: tm_base64gzip - Functions - Configuration Language
description: |-
  The tm_base64gzip function compresses the given string with gzip and then
  encodes the result in Base64.
---

# `tm_base64gzip` Function

`tm_base64gzip` compresses a string with gzip and then encodes the result in
Base64 encoding.

Terraform uses the "standard" Base64 alphabet as defined in
[RFC 4648 section 4](https://tools.ietf.org/html/rfc4648#section-4).

Strings in the Terraform language are sequences of unicode characters rather
than bytes, so this function will first encode the characters from the string
as UTF-8, then apply gzip compression, and then finally apply Base64 encoding.

While we do not recommend manipulating large, raw binary data in the Terraform
language, this function can be used to compress reasonably sized text strings
generated within the Terraform language. For example, the result of this
function can be used to create a compressed object in Amazon S3 as part of
an S3 website.

## Related Functions

* [`tm_base64encode`](./tm_base64encode.md) applies Base64 encoding _without_
  gzip compression.
* [`tm_filebase64`](./tm_filebase64.md) reads a file from the local filesystem
  and returns its raw bytes with Base64 encoding.
