# Locals Generation

Terramate provides a way to integrate its [globals](globals.md) and
[metadata](metadata.md) on Terraform code by exporting then as
[Terraform locals](https://www.terraform.io/language/values/locals) that
then can be referenced by any Terraform code inside the stack.

To make use of [globals](globals.md) and [metadata](metadata.md) define
a **export_as_locals** block on a [Terramate configuration file](config.md)
defining all locals that you want exported.

Given these globals defined somewhere inside the project:

```hcl
globals {
  data      = "data"
  more_data = "more data"
}
```

To use then directly on Terraform you can instruct code generation
to create them as locals:

```hcl
export_as_locals {
  data      = globals.data
  more_data = globals.more_data
}
```

Exporting using multiple blocks on the same [configuration file](config.md)
is also allowed, as far as no local is redefined, the previous example could
also be written like this:

```hcl
export_as_locals {
  data      = globals.data
}

export_as_locals {
  more_data = globals.more_data
}
```

Exported locals can be defined on any [configuration file](config.md)
so you can have core/base export locals definitions that will generate
locals for all stacks that are a sub directory of the [configuration](config.md).

Exported locals, just as [globals](globals.md), are evaluated on the context
of a stack, evaluation starts with exported locals defined on the stack itself
and then keeps going up on the file system until the project root is reached.

If exported locals don't have the same name, then they are just merged
together as they are evaluated going up on the project file system.

Exported locals can be overridden, just as [globals](globals.md), the most
specific exported locals override the more general ones, where by specific
we mean the definition closest to the stack being evaluated.

Given a project structured like this:

```
.
└── stacks
    ├── stack-1
    │   └── terramate.tm.hcl
    └── stack-2
        └── terramate.tm.hcl
```

Given a set of globals at the project root **./terramate.tm.hcl**:

```hcl
globals {
  data          = "data"
  more_data     = "more data"
  yet_more_data = "YMD"
}
```

If we define a [configuration file](config.md) for all stacks 
that export some locals at **stacks/terramate.tm.hcl**:

```hcl
export_as_locals {
  data      = global.data
  more_data = global.more_data
}
```

After running:

```sh
terramate generate
```

The following locals will be generated inside all stacks, creating two files:

* stacks/stack-1/_gen_locals.tm.tf
* stacks/stack-2/_gen_locals.tm.tf

Both with the same contents:

```hcl
locals {
  data      = "data"
  more_data = "more data"
}
```

So any Terraform code present on the stack can access these locals
as it would with any other manually defined locals:

```hcl
local.data
local.more_data
```

Now lets extend the exported locals only for **stack-1** by adding
this to **stacks/stack-1/terramate.tm.hcl**:

```hcl
export_as_locals {
  stack_1_only = globals.yet_more_data
}
```

Now the generated locals for **stacks/stack-1** will be:

```hcl
locals {
  data         = "data"
  more_data    = "more data"
  stack_1_only = "YMD"
}
```

While **stacks/stack-2** remained unchanged.

Now lets override the exported locals for **stack-1** by changing the
export definition on **stacks/stack-1/terramate.tm.hcl** to:

```hcl
export_as_locals {
  more_data = globals.yet_more_data
}
```

Now the generated locals for **stacks/stack-1** will be: 

```hcl
locals {
  data      = "data"
  more_data = "YMD"
}
```

You can also export Terramate [metadata](metadata.md) as locals:

```hcl
export_as_locals {
  stack_name = terramate.name
  stack_path = terramate.path
}
```

Interpolation and functions can be used:

```hcl
export_as_locals {
  interpolate = "${globals.data}-${globals.more_data}"
  functions   = split(" ", globals.more_data)
}
```

No namespace is created on Terramate for exported locals, so these are invalid:

```hcl
export_as_locals {
  something = export_as_locals.some_exported_name
}
```

```hcl
export_as_locals {
  something = local.some_exported_name
}
```
