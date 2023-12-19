---
title: tm_tostring - Functions - Configuration Language
description: The tm_tostring function converts a value to a string.
---

# `tm_tostring` Function

`tm_tostring` converts its argument to a string value.

Explicit type conversions are rarely necessary in Terraform because it will
convert types automatically where required. Use the explicit type conversion
functions only to normalize types returned in module outputs.

Only the primitive types (string, number, and bool) and `null` can be converted to string.
`tm_tostring(null)` produces a `null` value of type `string`. All other values produce an error. 

## Examples

```sh
tm_tostring("hello")
"hello"
tm_tostring(1)
"1"
tm_tostring(true)
"true"
tm_tostring(null)
tostring(null)
tm_tostring([])
Error: Invalid function argument

Invalid value for "v" parameter: cannot convert tuple to string.
```
