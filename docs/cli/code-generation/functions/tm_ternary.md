---
title: tm_ternary | Terramate Functions
description: |
    The tm_ternary is a replacement for the HCL ternary operator supporting
    partial evaluation and different expression types in each ternary branch.
---

# `tm_ternary` Function

This function is a replacement for HCL ternary operator `a ? b : c`. It circumvent
some limitations, like both expressions of the ternary producing values of the
same type. The `tm_ternary` function is not even limited to returning actual
values, it can also return expressions. Only the first boolean parameter must
be fully evaluated. If it is true, the first expression is returned, if it is
false the second expression is returned.

The function signature is:

```hcl
tm_ternary(bool, expr, expr) -> expr
```

## Examples 

```hcl
tm_ternary(false, access.data1, access.data2)
```

Will return the expression `access.data2`. While:

```hcl
tm_ternary(true, access.data1, access.data2)
```

Will return the expression `access.data1`.
