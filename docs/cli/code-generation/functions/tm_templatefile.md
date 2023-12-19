---
title: tm_templatefile - Functions - Configuration Language
description: |-
  The tm_templatefile function reads the file at the given path and renders its
  content as a template.
---

# `tm_templatefile` Function

`tm_templatefile` reads the file at the given path and renders its content
as a template using a supplied set of template variables.

```hcl
tm_templatefile(path, vars)
```

The template syntax is the same as for
[string templates](https://developer.hashicorp.com/terraform/language/expressions/strings#string-templates)
in the main Terramate language, including interpolation sequences delimited with
`${` ... `}`. This function just allows longer template sequences to be factored
out into a separate file for readability.

The "vars" argument must be an object. Within the template file, each of the
keys in the map is available as a variable for interpolation. The template may
also use any other function available in the Terramate language, except that
recursive calls to `tm_templatefile` are not permitted. Variable names must
each start with a letter, followed by zero or more letters, digits, or
underscores.

Strings in the Terramate language are sequences of Unicode characters, so
this function will interpret the file contents as UTF-8 encoded text and
return the resulting Unicode characters. If the file contains invalid UTF-8
sequences then this function will produce an error.

This function can be used only with files that already exist on disk at the
beginning of Terramate execution.

`*.tmtpl` is the recommended naming pattern to use for your template files.
Terramate will not prevent you from using other names, but following this
convention will help your editor understand the content and likely provide
better editing experience as a result.

## Examples

### Lists

Given a template file `backends.tmtpl` with the following content:

```sh
%{ for addr in ip_addrs ~}
backend ${addr}:${port}
%{ endfor ~}
```

The `tm_templatefile` function renders the template:

```sh
tm_templatefile("${path.module}/backends.tmtpl", { port = 8080, ip_addrs = ["10.0.0.1", "10.0.0.2"] })
backend 10.0.0.1:8080
backend 10.0.0.2:8080

```

### Maps

Given a template file `config.tmtpl` with the following content:

```sh
%{ for config_key, config_value in config }
set ${config_key} = ${config_value}
%{ endfor ~}
```

The `tm_templatefile` function renders the template:

```
tm_templatefile(
               "${path.module}/config.tmtpl",
               {
                 config = {
                   "x"   = "y"
                   "foo" = "bar"
                   "key" = "value"
                 }
               }
              )
set foo = bar
set key = value
set x = y
```

### Generating JSON or YAML from a template

If the string you want to generate will be in JSON or YAML syntax, it's
often tricky and tedious to write a template that will generate valid JSON or
YAML that will be interpreted correctly when using lots of individual
interpolation sequences and directives.

Instead, you can write a template that consists only of a single interpolated
call to either [`tm_jsonencode`](./tm_jsonencode.md) or
[`tm_yamlencode`](./tm_yamlencode.md), specifying the value to encode using
[normal Terraform expression syntax](https://developer.hashicorp.com/terraform/language/expressions)
as in the following examples:

```sh
${jsonencode({
  "backends": [for addr in ip_addrs : "${addr}:${port}"],
})}
```

```sh
${yamlencode({
  "backends": [for addr in ip_addrs : "${addr}:${port}"],
})}
```

Given the same input as the `backends.tmtpl` example in the previous section,
this will produce a valid JSON or YAML representation of the given data
structure, without the need to manually handle escaping or delimiters.
In the latest examples above, the repetition based on elements of `ip_addrs` is
achieved by using a
[`for` expression](https://developer.hashicorp.com/terraform/language/expressions/for)
rather than by using
[template directives](https://developer.hashicorp.com/terraform/language/expressions/strings#directives).

```json
{
  "backends": [
    "10.0.0.1:8080",
    "10.0.0.2:8080"
  ]
}
```

If the resulting template is small, you can choose instead to write
`tm_jsonencode` or `tm_yamlencode` calls inline in your main configuration files, and
avoid creating separate template files at all:

```hcl
locals {
  backend_config_json = jsonencode({
    "backends": [for addr in ip_addrs : "${addr}:${port}"],
  })
}
```

For more information, see the main documentation for
[`tm_jsonencode`](./tm_jsonencode.md) and [`tm_yamlencode`](./tm_yamlencode.md).

## Related Functions

* [`tm_file`](./tm_file.md) reads a file from disk and returns its literal contents
  without any template interpretation.
