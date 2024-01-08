---
title: tm_version_match | Terramate Functions
description: |
    The tm_version_match function checks if the version matches the provided constraint string.
---

# `tm_version_match` Function

`tm_version_match` returns `true` if `version` satisfies the `constraint` string.
By default **prereleases** are never matched if they're not explicitly provided
in the constraint.

The third parameter is an optional object of type below:

```hcl
{
  allow_prereleases: bool,
}
```

If `opt.allow_prereleases` is set to `true` then **prereleases** will be matched
accordingly to [Semantic Versioning](https://semver.org/) precedence rules.

The function signature is:

```hcl
tm_version_match(version:string, constraint:string, ...optional_arg:object) -> bool
```
