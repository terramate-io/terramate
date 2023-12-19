---
title: tm_vendor | Terramate Functions
description: |
    The tm_vendor function dynamically vendor modules during generation.
---

# `tm_vendor` Function

::: warning
This is an experimental function.
:::

`tm_vendor` takes a [Terraform module source](https://developer.hashicorp.com/terraform/language/modules/sources)
as a parameter and returns the local path of the given module source after it is
vendored. This function can only be used inside `generate_hcl` and
`generate_file` blocks. In the case of `generate_file` blocks it can only be
used when the context of the block is `stack`, it won't work with a `root` context.

The function signature is:

```hcl
tm_vendor(string) -> string
```

The function will work directly inside generated content
and also inside the `lets` block. The local path will be relative to the target directory
where code is being generated, which is determined by the `generate` block label.

For example:

```hcl
generate_hcl "file.hcl" {
  content {
    module "test" {
      source = tm_vendor("github.com/mineiros-io/terraform-google-service-account?ref=v0.1.0")
    }
  }
}
```

Will generate a local source relative to the stack directory, since the file is generated
directly inside a stack. But this:

```hcl
generate_hcl "dir/file.hcl" {
  content {
    module "test" {
      source = tm_vendor("github.com/mineiros-io/terraform-google-service-account?ref=v0.1.0")
    }
  }
}
```

Will generate a local source relative to `<stack-dir>/dir`, since the file is generated
inside a sub-directory of the stack.
