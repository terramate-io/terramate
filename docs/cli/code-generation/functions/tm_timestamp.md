---
title: tm_timestamp - Functions - Configuration Language
description: |-
  The tm_timestamp function returns a string representation of the current date
  and time.
---

# `tm_timestamp` Function

`tm_timestamp` returns a UTC timestamp string in [RFC 3339](https://tools.ietf.org/html/rfc3339) format.

In the Terraform language, timestamps are conventionally represented as
strings using [RFC 3339](https://tools.ietf.org/html/rfc3339)
"Date and Time format" syntax, and so `tm_timestamp` returns a string
in this format.

The result of this function will change every second, so using this function
directly with resource attributes will cause a diff to be detected on every
Terraform run. We do not recommend using this function in resource attributes,
but in rare cases it can be used in conjunction with
[the `ignore_changes` lifecycle meta-argument](https://developer.hashicorp.com/terraform/language/meta-arguments/lifecycle#ignore_changes)
to take the timestamp only on initial creation of the resource. For more stable
time handling, see the [Time Provider](https://registry.terraform.io/providers/hashicorp/time).

Due to the constantly changing return value, the result of this function cannot
be predicted during Terraform's planning phase, and so the timestamp will be
taken only once the plan is being applied.

## Examples

```sh
tm_timestamp()
2018-05-13T07:44:12Z
```

## Related Functions

* [`tm_formatdate`](./tm_formatdate.md) can convert the resulting timestamp to
  other date and time formats.
