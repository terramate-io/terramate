---
title: tm_one - Functions - Configuration Language
description: |-
  The tm_one function transforms a list with either zero or one elements into
  either a null value or the value of the first element.
---

# `tm_one` Function

`tm_one` takes a list, set, or tuple value with either zero or one elements.
If the collection is empty, `tm_one` returns `null`. Otherwise, `tm_one` returns
the first element. If there are two or more elements then `tm_one` will return
an error.

This is a specialized function intended for the common situation where a
conditional item is represented as either a zero- or one-element list, where
a module author wishes to return a single value that might be null instead.

For example:

```hcl
variable "include_ec2_instance" {
  type    = bool
  default = true
}

resource "aws_instance" "example" {
  count = var.include_ec2_instance ? 1 : 0

  # (other resource arguments...)
}

output "instance_ip_address" {
  value = tm_one(aws_instance.example[*].private_ip)
}
```

Because the `aws_instance` resource above has the `count` argument set to a
conditional that returns either zero or one, the value of
`aws_instance.example` is a list of either zero or one elements. The
`instance_ip_address` output value uses the `tm_one` function as a concise way
to return either the private IP address of a single instance, or `null` if
no instances were created.

## Relationship to the "Splat" Operator

The Terraform language has a built-in operator `[*]`, known as
[the _splat_ operator](https://developer.hashicorp.com/terraform/language/expressions/splat), and one of its functions
is to translate a primitive value that might be null into a list of either
zero or one elements:

```hcl
variable "ec2_instance_type" {
  description = "The type of instance to create. If set to null, no instance will be created."

  type    = string
  default = null
}

resource "aws_instance" "example" {
  count = length(var.ec2_instance_type[*])

  instance_type = var.ec2_instance_type
  # (other resource arguments...)
}

output "instance_ip_address" {
  value = one(aws_instance.example[*].private_ip)
}
```

In this case we can see that The `tm_one` function is, in a sense, the opposite
of applying `[*]` to a primitive-typed value. Splat can convert a possibly-null
value into a zero-or-one list, and `tm_one` can reverse that to return to a
primitive value that might be null.

## Examples

```sh
tm_one([])
null
tm_one(["hello"])
"hello"
tm_one(["hello", "goodbye"])

Error: Invalid function argument

Invalid value for "list" parameter: must be a list, set, or tuple value with
either zero or one elements.
```

### Using `tm_one` with sets

The `tm_one` function can be particularly helpful in situations where you have a
set that you know has only zero or one elements. Set values don't support
indexing, so it's not valid to write `var.set[0]` to extract the "first"
element of a set, but if you know that there's only one item then `tm_one` can
isolate and return that single item:

```sh
tm_one(toset([]))
null
tm_one(toset(["hello"]))
"hello"
```

Don't use `tm_one` with sets that might have more than one element. This function
will fail in that case:

```sh
tm_one(tm_toset(["hello","goodbye"]))

Error: Invalid function argument

Invalid value for "list" parameter: must be a list, set, or tuple value with
either zero or one elements.
```
