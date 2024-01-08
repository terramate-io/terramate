---
title: tm_fileset - Functions - Configuration Language
description: The tm_fileset function enumerates a set of regular file names given a pattern.
---

# `tm_fileset` Function

`tm_fileset` enumerates a set of regular file names given a path and pattern.
The path is automatically removed from the resulting set of file names and any
result still containing path separators always returns forward slash (`/`) as
the path separator for cross-system compatibility.

```hcl
tm_fileset(path, pattern)
```

Supported pattern matches:

- `*` - matches any sequence of non-separator characters
- `**` - matches any sequence of characters, including separator characters
- `?` - matches any single non-separator character
- `{alternative1,...}` - matches a sequence of characters if one of the comma-separated alternatives matches
- `[CLASS]` - matches any single non-separator character inside a class of characters (see below)
- `[^CLASS]` - matches any single non-separator character outside a class of characters (see below)

Note that the doublestar (`**`) must appear as a path component by itself. A
pattern such as /path\*\* is invalid and will be treated the same as /path\*, but
/path\*/\*\* should achieve the desired result.

Character classes support the following:

- `[abc]` - matches any single character within the set
- `[a-z]` - matches any single character within the range

Functions are evaluated during configuration parsing rather than at apply time,
so this function can only be used with files that are already present on disk
before Terraform takes any actions.

## Examples

```sh
tm_fileset(path.module, "files/*.txt")
[
  "files/hello.txt",
  "files/world.txt",
]

tm_fileset(path.module, "files/{hello,world}.txt")
[
  "files/hello.txt",
  "files/world.txt",
]

tm_fileset("${path.module}/files", "*")
[
  "hello.txt",
  "world.txt",
]

tm_fileset("${path.module}/files", "**")
[
  "hello.txt",
  "world.txt",
  "subdirectory/anotherfile.txt",
]
```
