# Terraform Code Generation

Terramate provides a way to integrate its [globals](globals.md) and
[metadata](metadata.md) in Terraform code by allowing you to generate
arbitrary Terraform code that leverages Terramate data.
The generated code can then be composed/referenced by any Terraform code
inside a stack.

Terraform code generation starts with the definition of a `export_as_terraform`
block in a [Terramate configuration file](config.md) defining the code you
want to generate inside the block. The code may include:

* Blocks, sub blocks, etc 
* Attributes initialized by literals
* Terramate Global references
* Terramate Metadata references
* Expressions using interpolation, functions, etc

Mostly of what you can do on Terraform can be done on a `export_as_terraform`
block, for now only the following is disallowed:

* References to variables on the form `var.name`
* References to locals on the form `local.name`

Basically there is no support for partial evaluation (yet), so anything defined
needs to be evaluated on the context of the code generation and the final generated
code will have the results of the evaluation.

Each `export_as_terraform` block requires a label. This label is part of the identity
of the block and is also used as a default to which filename will be used when
code is generated. Given a label `x` the filename will be `_gen_terramate_x.tf`. The labels are
also used to configure different filenames for each block if the default names are
undesired, more details on how to configure this can be checked [here](todo-docs-for-config).

Now lets jump to some examples. Lets generate backend and provider configuration
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
stacks by defining a `export_as_terraform` blocks on the root
of the project:

```hcl
export_as_terraform "backend" {
  backend "local" {
    param = global.backend_data
  }
}
```

Which will generate code for all stacks using the filename `_gen_terramate_backend.tf`:

```hcl
backend "local" {
  param = "backend_data"
}
```

To generate provider/Terraform configuration for all stacks we can add
on the root configuration:

```hcl
export_as_terraform "provider" {

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
```

Which will generate code for all stacks using the filename `_gen_terramate_provider.tf`:

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

TODO: Define overriding behavior (hierarchies).
