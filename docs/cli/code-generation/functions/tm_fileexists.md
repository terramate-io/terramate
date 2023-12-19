---
title: tm_fileexists - Functions - Configuration Language
description: The tm_fileexists function determines whether a file exists at a given path.
---

# `tm_fileexists` Function

`tm_fileexists` determines whether a file exists at a given path.

```hcl
tm_fileexists(path)
```

Functions are evaluated during configuration parsing rather than at apply time,
so this function can only be used with files that are already present on disk
before Terraform takes any actions.

This function works only with regular files. If used with a directory, FIFO,
or other special mode, it will return an error.

## Examples

```sh
tm_fileexists("${path.module}/hello.txt")
true
```

```hcl
tm_fileexists("custom-section.sh") ? file("custom-section.sh") : local.default_content
```

## Related Functions

* [`tm_file`](./tm_file.md) reads the contents of a file at a given path
