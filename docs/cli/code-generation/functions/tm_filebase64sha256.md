---
title: tm_filebase64sha256 - Functions - Configuration Language
description: |-
  The tm_filebase64sha256 function computes the SHA256 hash of the contents of
  a given file and encodes it with Base64.
---

# `tm_filebase64sha256` Function

`tm_filebase64sha256` is a variant of [`tm_base64sha256`](./tm_base64sha256.md)
that hashes the contents of a given file rather than a literal string.

This is similar to `tm_base64sha256(file(filename))`, but
because [`tm_file`](./tm_file.md) accepts only UTF-8 text it cannot be used to
create hashes for binary files.
