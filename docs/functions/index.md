---
title: Functions
description: Terramate provides the same built-in functions as Terraform  but prefixed with tm_.

prev:
  text: 'Generate File'
  link: '/code-generation/generate-file'

next:
  text: 'tm_ternary'
  link: '/functions/tm_ternary'
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
