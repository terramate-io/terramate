---
title: tm_hcl_expression | Terramate Functions
description: This function is capable of generating HCL expressions from a string.

prev:
  text: 'tm_ternary'
  link: '/functions/tm_ternary.md'

next:
  text: 'tm_version_match'
  link: 'functions/tm_version_match.md'
---

# `tm_hcl_expression` Function

This function receives a string as parameter and return the string
contents as an expression. It is particularly useful to circumvent some
limitations on HCL and Terraform when building complex expressions from
dynamic data.

Since this function produces an expression, not a final evaluated value,
it is only allowed to be used on contexts where partial evaluation is
allowed, which currently is only the `generate_hcl.content` block.

The function signature is:

```
tm_hcl_expression(string) -> expr
```

## Examples

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
