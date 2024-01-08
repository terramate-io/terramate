---
title: tm_file - Functions - Configuration Language
description: |-
  The tm_file function reads the contents of the file at the given path and
  returns them as a string.
---

# `tm_file` Function

`tm_file` reads the contents of a file at the given path and returns them as a string.

```hcl
tm_file(path)
```

Strings in the Terraform language are sequences of Unicode characters, so
this function will interpret the file contents as UTF-8 encoded text and
return the resulting Unicode characters. If the file contains invalid UTF-8
sequences then this function will produce an error.

This function can be used only with files that already exist on disk
at the beginning of a Terraform run. Functions do not participate in the
dependency graph, so this function cannot be used with files that are generated
dynamically during a Terraform operation. We do not recommend using dynamic
local files in Terraform configurations, but in rare situations where this is
necessary you can use
[the `local_file` data source](https://registry.terraform.io/providers/hashicorp/local/latest/docs/data-sources/file)
to read files while respecting resource dependencies.

## Examples

```sh
tm_file("${path.module}/hello.txt")
Hello World
```

## Related Functions

* [`tm_filebase64`](./tm_filebase64.md) also reads the contents of a given file,
  but returns the raw bytes in that file Base64-encoded, rather than
  interpreting the contents as UTF-8 text.
* [`tm_fileexists`](./tm_fileexists.md) determines whether a file exists
  at a given path.
* [`tm_templatefile`](./tm_templatefile.md) renders using a file from disk as a
  template.
