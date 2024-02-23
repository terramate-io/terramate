---
title: tm_filesha256 - Functions - Configuration Language
description: |-
  The tm_filesha256 function computes the SHA256 hash of the contents of
  a given file and encodes it as hex.
---

# `tm_filesha256` Function

`tm_filesha256` is a variant of [`tm_sha256`](./tm_sha256.md)
that hashes the contents of a given file rather than a literal string.

This is similar to `tm_sha256(tm_file(filename))`, but
because [`tm_file`](./tm_file.md) accepts only UTF-8 text it cannot be used to
create hashes for binary files.
