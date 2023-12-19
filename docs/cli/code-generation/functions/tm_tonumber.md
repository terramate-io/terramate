---
title: tm_tonumber - Functions - Configuration Language
description: The tm_tonumber function converts a value to a number.
---

# `tm_tonumber` Function

`tm_tonumber` converts its argument to a number value.

Explicit type conversions are rarely necessary in Terraform because it will
convert types automatically where required. Use the explicit type conversion
functions only to normalize types returned in module outputs.

Only numbers, `null`, and strings containing decimal representations of numbers can be
converted to number. All other values will produce an error.

## Examples

```sh
tm_tonumber(1)
1
tm_tonumber("1")
1
tm_tonumber(null)
null
tm_tonumber("no")
Error: Invalid function argument

Invalid value for "v" parameter: cannot convert "no" to number: string must be
a decimal representation of a number.
```
