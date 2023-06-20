---
title: Functions | Terramate
description: Terramate adds powerful capabilities such as code generation, stacks, orchestration, change detection, data sharing and more to Terraform.

prev:
  text: 'Generate File'
  link: '/code-generation/generate-file'

next:
  text: 'Configuration'
  link: '/configuration/'
---

# Terramate functions
One of its key features is the availability of Terramate functions, which
enhance the capabilities of Terraform's built-in functions. In this section,
we will provide an overview of Terramate functions as [Terraform v0.15.13](https://www.terraform.io/language/functions), and discuss their usage
with the `tm_` prefix. Additionally, we will introduce the concept of custom
Terramate functions.

## Overview of Terramate functions
Functions are similar to Terraform's built-in functions but come with the
added benefit of extended functionality and also provide flexibility. They
allow you to manipulate the data, perform calculations, and make decisions
within your Terramate configurations. Thus, they give you the leverage to
streamline your infrastructure code and make it more efficient.

## Usage of Terramate functions with the `tm_` prefix
As Terramate functions are similar to Terraform's built-in functions, to
differentiate Terramate functions from Terraform's native functions,
Terramate functions are prefixed with `tm_`. This prefix helps in easily
identifying and distinguishing Terramate functions within the codebase.

## Introduction to custom Terramate functions
In addition to the built-in Terramate functions inherited from Terraform,
Terramate also offers the ability to define custom functions tailored to your
specific needs. This empowers you to create reusable functions and abstract
complex logic, improving the maintainability and modularity of your Terramate
configurations.

In the following sections, we will explore some of the essential built-in
functions and experimental functions offered by Terramate.

## Built-in Terramate functions
Terramate provides several built-in functions that can be used in your
Terramate configurations. These functions offer extended capabilities and
flexibility when working with infrastructure. Let's explore some of the key
built-in Terramate functions.


### `tm_try(global.b, null)`
The `tm_try` function is similar to Terraform's `try` function and allows you
to handle situations where a value may or may not be available. It takes two
arguments.

Example:

```hcl
globals {
  a = tm_try(global.b, null)
}
```

In the above example, the `tm_try` function is used to assign the value of
`global.b` to the variable a. If `global.b` is set, its value will be
assigned to `a`. Otherwise, `a` will be assigned the value null.

To define each function prototype we use with a small pseudo language
where each parameter is defined just with its type and `-> type` to
indicate a return type, if any.

Most types are self explanatory, one special type though would be
`expr`. When `expr` is used it means an expression that may not be evaluated
into a value of a specific type. This is important for functions that uses
partially evaluated expressions as parameters and may return expressions
themselves.

### `tm_ternary(bool,expr,expr) -> expr`
The `tm_ternary` function provides a replacement for the ternary operator `a ? b : c`
commonly used in programming languages. It takes three arguments: a boolean condition,
an expression to evaluate if the condition is `true`, and an expression to evaluate if
the condition is `false`. The function returns the evaluated expression based on the
condition.

The `tm_ternary` function is not even limited to returning actual values, it can also
return expressions. Only the first boolean parameter must be fully evaluated. If it is
`true`, the first expression is returned, if it is `false` the second expression is 
returned.

For example

```hcl
tm_ternary(false, access.data1, access.data2)
```

In the above example, the `tm_ternary` function evaluates the condition and returns
`access.data2` because the condition is `false`. Similarly, for the following:

```hcl
tm_ternary(true, access.data1, access.data2)
```

It will return `access.data1` because the condition is `true`.

### `tm_hcl_expression(string) -> expr`
The `tm_hcl_expression` function is particularly useful to circumvent some limitations
on HCL and for constructing complex expressions dynamically. It takes a string as input
and returns the string contents as an expression. This function can be used when
building HCL expressions using dynamic data.

Since this function produces an expression, not a final evaluated value, it is only
allowed to be used on contexts where partial evaluation is allowed, which currently is
only the `generate_hcl.content` block.

To understand the use of `tm_hcl_expression`, more clearly,  lets say we have a global
named data defined like this:

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

In the above example, the `tm_hcl_expression` function is used to generate a complex
expression by concatenating the string `data.google_active_folder._parent_id.id.` with
the value of `global.data`. This allows for dynamic expression building based on the
provided string input that will look like this:

```hcl
expr = data.google_active_folder._parent_id.id.data
```

### `tm_version_match(version:string, constraint:string, ...optional_arg:object)`
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

## Experimental functions

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

For example

```hcl
generate_hcl "file.hcl" {
  content {
    module "test" {
      source = tm_vendor("github.com/mineiros-io/terraform-google-service-account?ref=v0.1.0")
    }
  }
}
```

In the above example, the `tm_vendor` function is used to obtain the local source of a
Terraform module from a specific GitHub repository. The resulting local source path is
relative to the stack directory since the file is generated directly within the stack.

But in the following:

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

> Note: It is important to keep in mind that the experimental functions may undergo
changes and improvements in future versions of Terramate. Therefore, it's recommended
to consult the official documentation and release notes for any updates or
modifications to these functions.
>