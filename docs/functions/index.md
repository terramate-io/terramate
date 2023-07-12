---
title: Functions
description: Terramate adds powerful capabilities such as code generation, stacks, orchestration, change detection, data sharing and more to Terraform.

prev:
  text: 'Generate File'
  link: '/code-generation/generate-file'

next:
  text: 'Configuration'
  link: '/configuration/'
---

# Terramate Functions

Terramate provides the same built-in functions as
[Terraform v0.15.13](https://www.terraform.io/language/functions) but prefixed with `tm_`.
For example, to use the try function when evaluating a global:

```hcl
globals {
  a = tm_try(global.b, null)
}
```

Will work exactly as Terraform's `try` function.
Terramate also provides some custom functions of its own.

To define each function prototype we use with a small pseudo language
where each parameter is defined just with its type and `-> type` to
indicate a return type, if any.

Most types are self explanatory, one special type though would be
`expr`. When `expr` is used it means an expression that may not be evaluated
into a value of a specific type. This is important for functions that uses
partially evaluated expressions as parameters and may return expressions
themselves.


## `tm_ternary(bool,expr,expr) -> expr`

This function is a replacement for HCL ternary operator `a ? b : c`. It circumvent
some limitations, like both expressions of the ternary producing values of the
same type. The `tm_ternary` function is not even limited to returning actual
values, it can also return expressions. Only the first boolean parameter must
be fully evaluated. If it is true, the first expression is returned, if it is
false the second expression is returned.

For example:

```hcl
tm_ternary(false, access.data1, access.data2)
```

Will return the expression `access.data2`. While:

```hcl
tm_ternary(true, access.data1, access.data2)
```

Will return the expression `access.data1`.


## `tm_hcl_expression(string) -> expr`

This function receives a string as parameter and return the string
contents as an expression. It is particularly useful to circumvent some
limitations on HCL and Terraform when building complex expressions from
dynamic data.

Since this function produces an expression, not a final evaluated value,
it is only allowed to be used on contexts where partial evaluation is
allowed, which currently is only the `generate_hcl.content` block.

To use `tm_hcl_expression`, lets say we have a global named data defined like this:

```hcl
globals {
  data = "data"
}
```

You can use this global to build a complex expression when generation code,
like this:

```hcl
generate_hcl "test.hcl" {
    content {
        expr = tm_hcl_expression("data.google_active_folder._parent_id.id.${global.data}")
    }
}
```

Which will generate:

```hcl
expr = data.google_active_folder._parent_id.id.data
```

## `tm_version_match(version:string, constraint:string, ...optional_arg:object)`

This function returns `true` if `version` satisfies the `constraint` string.
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

## Experimental Functions

These functions are experimental and some of them may only be available on
specific contexts.

### `tm_vendor(string) -> string`

Receives a [Terraform module source](https://developer.hashicorp.com/terraform/language/modules/sources)
as a parameter and returns the local path of the given module source after it is
vendored. This function can only be used inside `generate_hcl` and
`generate_file` blocks. In the case of `generate_file` blocks it can only be
used when the context of the block is `stack`, it won't work with a `root` context.

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
