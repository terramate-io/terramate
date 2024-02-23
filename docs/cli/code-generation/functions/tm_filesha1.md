---
title: tm_filesha1 - Functions - Configuration Language
description: |-
  The tm_filesha1 function computes the SHA1 hash of the contents of
  a given file and encodes it as hex.
---

# `tm_filesha1` Function

`tm_filesha1` is a variant of [`tm_sha1`](./tm_sha1.md)
that hashes the contents of a given file rather than a literal string.

This is similar to `tm_sha1(tm_file(filename))`, but
because [`tm_file`](./tm_file.md) accepts only UTF-8 text it cannot be used to
create hashes for binary files.
