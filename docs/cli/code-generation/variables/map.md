---
title: Map Block
description: The map block can be used to create complex maps and objects inside globals and lets blocks.
---

# Map Block

The `map` block can be used to create complex maps/objects inside
[Globals](globals.md) and [Lets](lets.md) blocks.
It can be used to aggregate lists of objects into maps that have duplicated keys
and need a defined way of deep merging values of the same key.

The following is a very basic example introducing the `map` block inside a `globals` block:

```hcl
globals {
  map obj {
    for_each = [
      { name = "a", value = 5 },
      { name = "c", value = 0 },
      { name = "a", value = 15 },
      { name = "b", value = 5 },
      { name = "c", value = 20 },
    ]

    iterator = elem
    key      = elem.new.name

    value {
      value    = elem.new.value
      previous = tm_try(elem.old.value, null)
    }
  }
}
```

The code above will result in the `global.obj` defined below:

```hcl
obj = {
  a = {
    previous = 5
    value    = 15
  }
  b = {
    previous = null
    value    = 5
  }
  c = {
    previous = 0
    value    = 20
  }
}
```

The `map` will iterate over the values in `for_each`, setting the iterator variable (`elem` in this case, `element` by
default) with the `elem.new` and `elem.old` values for the provided `key`. The `elem.new` contains the current iterated
value and the `elem.old` contains the values of the previous iteration of the current `key`.

Now it's time for a more useful example.

Take a look at the `global.orders` list declared below:

```hcl
globals {
  orders = [
    { name = "Morpheus", product = "sunglass", price = 100.5 },
    { name = "Trinity", product = "cape", price = 82.30 },
    { name = "Trinity", product = "necklace", price = 25.0 },
    { name = "Trinity", product = "sunglass", price = 100.5 },
    { name = "Anderson", product = "ollydbg", price = 30 },
    { name = "Morpheus", product = "boot", price = 65 },
    { name = "Anderson", product = "cape", price = 82.30 },
    { name = "Morpheus", product = "sunglass", price = 145.50 },
  ]
}
```

Let's aggregate those values by `name`, by using the `map` block as defined below:

```hcl
globals {
  map totals {
    for_each = global.orders

    key = element.new.name

    value {
      total_spent = tm_try(element.old.total_spent, 0) + element.new.price
    }
  }
}
```

Which will result in the global object below:

```hcl
totals = {
  Anderson = {
    total_spent = 112.3
  }
  Morpheus = {
    total_spent = 311
  }
  Trinity = {
    total_spent = 207.8
  }
}
```

Using nested `map` blocks and then aggregating by `product` for each `name` can be achieved using the following:

Example:

```hcl
globals {
  map totals {
    for_each = global.orders

    iterator = per_name

    key = per_name.new.name

    value {
      total_spent = tm_try(per_name.old.total_spent, 0) + per_name.new.price

      map per_product {
        for_each = [for v in global.orders : v if v.name == per_name.new.name]

        iterator = per_product

        key = per_product.new.product

        value {
          total = tm_try(per_product.old.total, 0) + per_product.new.price
        }
      }
    }
  }
}
```

which will result the object below:

```hcl
obj = {
  Anderson = {
    total_spent = 112.3
    per_product = {
      cape = {
        total = 82.3
      }
      ollydbg = {
        total = 30
      }
    }
  }
  Morpheus = {
    total_spent = 311
    per_product = {
      boot = {
        total = 65
      }
      sunglass = {
        total = 246
      }
    }
  }
  Trinity = {
    total_spent = 207.8
    per_product = {
      cape = {
        total = 82.3
      }
      necklace = {
        total = 25
      }
      sunglass = {
        total = 100.5
      }
    }
  }
}
```
