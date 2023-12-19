---
title: tm_tobool - Functions - Configuration Language
description: The tm_tobool function converts a value to boolean.
---

# `tm_tobool` Function

`tm_tobool` converts its argument to a boolean value.

Explicit type conversions are rarely necessary in Terraform because it will
convert types automatically where required. Use the explicit type conversion
functions only to normalize types returned in module outputs.

Only boolean values, `null`, and the exact strings `"true"` and `"false"` can be
converted to boolean. All other values will produce an error.

## Examples

```sh
tm_tobool(true)
true
tm_tobool("true")
true
tm_tobool(null)
null
tm_tobool("no")
Error: Invalid function argument

Invalid value for "v" parameter: cannot convert "no" to bool: only the strings
"true" or "false" are allowed.

tm_tobool(1)
Error: Invalid function argument

Invalid value for "v" parameter: cannot convert number to bool.
```
