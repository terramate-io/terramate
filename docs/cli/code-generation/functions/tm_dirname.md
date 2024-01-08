---
title: tm_dirname - Functions - Configuration Language
description: The tm_dirname function removes the last portion from a filesystem path.
---

# `tm_dirname` Function

`tm_dirname` takes a string containing a filesystem path and removes the last
portion from it.

This function works only with the path string and does not access the
filesystem itself. It is therefore unable to take into account filesystem
features such as symlinks.

If the path is empty then the result is `"."`, representing the current
working directory.

The tm_behavior of this function depends on the host platform. On Windows systems,
it uses backslash `\` as the path segment separator. On Unix systems, the slash
`/` is used. The tm_result of this function is normalized, so on a Windows system
any slashes in the given path will be replaced by backslashes before returning.

Referring directly to filesystem paths in resource arguments may cause
spurious diffs if the same configuration is applied from multiple systems or on
different host operating systems. We recommend using filesystem paths only
for transient values, such as the argument to [`tm_file`](./tm_file.md) (where
only the contents are then stored) or in `connection` and `provisioner` blocks.

## Examples

```sh
tm_dirname("foo/bar/baz.txt")
foo/bar
```

## Related Functions

* [`tm_basename`](./tm_basename.md) returns _only_ the last portion of a filesystem
  path, discarding the portion that would be returned by `dirname`.
