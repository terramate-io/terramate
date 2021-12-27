# Locals Generation

Terramate provides a way to integrate its [globals](globals.md) and
[metadata](metadata.md) on Terraform code by exporting then as
[Terraform locals](https://www.terraform.io/language/values/locals) that
then can be referenced by any Terraform code inside the stack.

To make use of [globals](globals.md) and [metadata](metadata.md) define
a **generate** block inside the **stack** block, defining all locals
that you want exported.

Given these globals defined somewhere inside the project:

```hcl
globals {
  data      = "data"
  more_data = "more data"
}
```

To use then directly on Terraform you can instruct code generation
to create them as locals for a stack:

```hcl
stack {
  generate {
    locals {
      data      = globals.data
      more_data = globals.more_data
    }
  }
}
```

After running:

```sh
terramate generate
```

The following locals will be generated inside the stack:

```hcl
locals {
  data      = "data"
  more_data = "more data"
}
```

You can do the same for Terramate [metadata](metadata.md):

```hcl
stack {
  generate {
    locals {
      stack_name = terramate.name
      stack_path = terramate.path
    }
  }
}
```

Interpolation and functions can be used:

```hcl
stack {
  generate {
    locals {
      interpolate = "${globals.data}-${globals.more_data}"
      functions   = split(" ", globals.more_data)
    }
  }
}
```

Generates:

```hcl
locals {
  interpolate = "data-more data"
  functions   = ["more", "data"]
}
```
