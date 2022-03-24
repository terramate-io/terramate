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

Which will generate code for all stacks, creating a file named `backend.tf`:

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

Which will generate code for all stacks using the filename `provider.tf`:

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
