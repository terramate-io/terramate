# HCL Code Generation

Terramate supports the generation of arbitrary HCL code referencing 
[Terramate defined data](../sharing-data.md).

The generated code can then be composed/referenced by any Terraform code
inside a stack (or any other tool that uses HCL, like [Packer](https://www.packer.io/)).

HCL code generation is done using `generate_hcl`
blocks in [Terramate configuration files](../config-overview.md).

The code may include:

* Blocks, sub blocks, etc 
* Attributes initialized by literals
* Terramate Global references
* Terramate Metadata references
* Expressions using interpolation, functions, etc

Anything you can do in Terraform can be generated using a `generate_hcl`
block. References to Terramate globals and metadata are evaluated, but any
other reference is just transported to the generated code (partial evaluation).

Each `generate_hcl` block requires a single label.
This label is the filename of the generated code, multiple `generate_hcl` blocks
with the same label/filename will result in an error.

Inside the `generate_hcl` block a `content` block is required.
All code inside `content` is going to be used to generate the final HCL code.

Now lets jump to some examples. Lets generate backend and provider configurations
for all stacks inside a project.

Given these globals defined on the root of the project:

```hcl
globals {
  backend_data = "backend_data"
  provider_data = "provider_data"
  provider_version = "0.6.6"
  terraform_version = "1.1.3"
}
```

We can define the generation of a backend configuration for all
stacks by defining a `generate_hcl` block in the root of the project:

```hcl
generate_hcl "backend.tf" {
  content {
    backend "local" {
      param = global.backend_data
    }
  }
}
```

Which will generate code for all stacks, creating a file named `backend.tf` on each stack:

```hcl
backend "local" {
  param = "backend_data"
}
```

To generate provider/terraform configuration for all stacks we can add
in the root configuration:

```hcl
generate_hcl "provider.tf" {

  content {
    provider "name" {
      param = global.provider_data
    }

    terraform {
      required_providers {
        name = {
          source  = "integrations/name"
          version = global.provider_version
        }
      }
    }

    terraform {
      required_version = global.terraform_version
    }

  }

}
```

Which will generate code for all stacks, creating a file named `provider.tf` on each stack:

```hcl
provider "name" {
  param = "provider_data"
}

terraform {
  required_providers {
    name = {
      source  = "integrations/name"
      version = "0.6.6"
    }
  }
}

terraform {
  required_version = "1.1.3"
}
```

## Hierarchical Code Generation

HCL code generation can be defined anywhere inside a project, from a specific
stack, which defines code generation only for the specific stack, to parent dirs
or even the project root, which then has the potential to affect code generation
to multiple or all stacks (as seen in the previous example).

There is no overriding or merging behavior for `generate_hcl` blocks.
Blocks defined at different levels with the same label aren't allowed, resulting
in failure for the overall code generation process.


## Conditional Code Generation

Conditional code generation is achieved by the use of the `condition` attribute.
The `condition` attribute should always evaluate to a boolean. The file will
be generated only if it evaluates to **true**.

If the `condition` attribute is absent then it is assumed to be always true.

Any expression that produces a boolean can be used, including references
to globals and function calls. For example:

```hcl
generate_hcl "file" {
  condition = tm_length(global.list) > 0
  content {
    locals {
      list = global.list
    }
  }
}
```

Will only generate the file for stacks that the expression
`tm_length(global.list) > 0` evaluates to true.


## Partial Evaluation

A partial evaluation strategy is used when generating HCL code.
This means that you can generate code with unknown references/function calls
and those will be copied verbatim to the generated code.

Lets assume we have a single global as Terramate data:

```hcl
globals {
  terramate_data = "terramate_data"
}
```

And we want to mix this Terramate references with Terraform
references, like locals/vars/outputs/etc.
All we have to do is define our `generate_hcl` block like this:


```hcl
generate_hcl "main.tf" {
  content {
    resource "myresource" "name" {
      count = var.enabled ? 1 : 0
      data  = global.terramate_data
      path  = terramate.path
      name  = local.name
    }
  }
}
```

And it will generate the following `main.tf` file:

```
resource "myresource" "name" {
  count = var.enabled ? 1 : 0
  data  = "terramate_data"
  path  = "/path/to/stack"
  name  = local.name
}
```

The `global.terramate_data` and `terramate.path` references were evaluated,
but the references to `var.enabled` and `local.name` were retained as is,
hence the partial evaluation.

Function calls are also partially evaluated. Any unknown function call
will be retained as is, but any function call starting with the prefix
`tm_` is considered a Terramate function and will be evaluated.
Terramate function calls can only have as parameters Terramate references
or literals.

For example, given:

```hcl
generate_hcl "main.tf" {
  content {
    resource "myresource" "name" {
      data  = tm_upper(global.terramate_data)
      name  = upper(local.name)
    }
  }
}
```

This will be generated:

```hcl
resource "myresource" "name" {
  data  = "TERRAMATE_DATA"
  name  = upper(local.name)
}
```

If one of the parameters of a unknown function call is a Terramate
reference the value of the Terramate reference will be replaced on the
function call.

This:

```hcl
generate_hcl "main.tf" {
  content {
    resource "myresource" "name" {
      data  = upper(global.terramate_data)
      name  = upper(local.name)
    }
  }
}
```

Generates:

```hcl
generate_hcl "main.tf" {
  content {
    resource "myresource" "name" {
      data  = upper("terramate_data")
      name  = upper(local.name)
    }
  }
}
```

Currently there is no partial evaluation of `for` expressions.
Referencing Terramate data inside a `for` expression will result
in an error (`for` expressions with unknown references are copied as is).
