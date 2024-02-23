---
title: tm_filemd5 - Functions - Configuration Language
description: |-
  The tm_filemd5 function computes the MD5 hash of the contents of
  a given file and encodes it as hex.
---

# `tm_filemd5` Function

`tm_filemd5` is a variant of [`tm_md5`](./tm_md5.md)
that hashes the contents of a given file rather than a literal string.

This is similar to `tm_md5(tm_file(filename))`, but
because [`tm_file`](./tm_file.md) accepts only UTF-8 text it cannot be used to
create hashes for binary files.
