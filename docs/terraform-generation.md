# Terraform Code Generation

Terramate supports the generation of arbitrary Terraform code using 
both [globals](globals.md) and [metadata](metadata.md).
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

Most of what you can do in Terraform can be done in a `export_as_terraform`
block. For now, only the following is not allowed:

* References to variables on the form `var.name`
* References to locals on the form `local.name`

Basically there is no support for partial evaluation (yet), so anything defined
needs to be evaluated in the context of the code generation, and the final generated
code will have the results of the evaluation. This means that any function calls,
file reading functions, references to globals/metadata, will all be evaluated
at code generation time and the generated code will only have literals like strings,
numbers, lists, maps, objects, etc.

Each `export_as_terraform` block requires a label. This label is part of the identity
of the block and is also used as a default for which filename will be used when
code is generated. Given a label `x` the filename will be `_gen_terramate_x.tf`. The labels are
also used to configure different filenames for each block if the default names are
undesired. More details on how to configure this can be checked [here](todo-docs-for-config).

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
stacks by defining a `export_as_terraform` block in the root
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
in the root configuration:

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

## Hierarchical Code Generation

Terraform code generation can be defined anywhere inside a project, from a specific
stack, which defines code generation only for the specific stack, to parent dirs
or even the project root, which then has the potential to affect code generation
to multiple or all stacks (as seen on the previous example).

This does raise the question of how code generation configuration is merged/overridden
as Terramate navigate the project loading and evaluating configuration in order to
generate code for each stack.

In order to explain how this works, lets define the concept of specific vs general
configuration. The closer a configuration is to an actual stack, the more specific it
is, the closer it is to the root of the project the more general it is.

For example, given a stack `stacks/stack-1`, here is the order from more specific
to more general:

* `stacks/stack-1`
* `stacks`
* `/` which means the project root

Given this definition, the behavior of `export_as_terraform` blocks is that
more specific configuration always override general purpose configuration.
There is no merge strategy/ composition involved, the configuration found
closest to a stack on the file system, or directly at the stack directory,
is the one used, ignoring more general configuration.

It is important to note that overriding happens when `export_as_terraform`
blocks are considered the same, and the identity of a `export_as_terraform`
block includes its label. Lets use as an example the
previously mentioned `stacks/stack-1`.

Given this configuration at `stacks/terramate.tm.hcl`:

```hcl
export_as_terraform "provider" {
  terraform {
    required_version = "1.1.13"
  }
}
```

And this configuration at `stacks/stack-1/terramate.tm.hcl`:

```hcl
export_as_terraform "backend" {
  backend "local" {
    param = "example"
  }
}
```

No overriding happens since each block has a different label and will generate
its own code in a separated file.

But if we had this configuration at `stacks/stack-1/terramate.tm.hcl`:

```hcl
export_as_terraform "provider" {
  terraform {
    required_version = "overriden"
  }
}
```

Now for `stacks/stack-1` the generated code would be:

```hcl
terraform {
  required_version = "overriden"
}
```

Since the `stacks/stack-1` configuration is overriding the previous
definition at `stacks`. Any other stack under `stacks` would remain
with the configuration defined on the parent dir `stacks`.

The overriding is total, there is no merging involved on the blocks inside
`export_as_terraform`, so if a parent directory defines a
configuration like this:

```hcl
export_as_terraform "name" {
    block1 {
    }
    block2 {
    }
    block3 {
    }
}
```

And a more specific configuration redefines it like this:

```hcl
export_as_terraform "name" {
    block4 {
    }
}
```

The final result is:

```hcl
block4 {
}
```
