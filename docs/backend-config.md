# Backend Configuration

Terramate provides some facilities to improve on how backend configuration
is managed on Terraform. The idea is to circumvent some of the limitations
from Terraform that makes it really hard to avoid duplication/mistakes
when managing backend configuration.

There is no way to define a single parametrized backend configuration
that can then be re-used across different stacks/environments/etc.

You can't even use a local variable as a parameter on the backend config,
from [Terraform docs](https://www.terraform.io/docs/language/settings/backends/configuration.html):

```
A backend block cannot refer to named values
(like input variables, locals, or data source attributes).
```

With those limitations in mind, terramate provides a way to:

* Define a single parametrized backend config and re-use it on multiple stacks.
* Use terramate metadata, like stack name/path, on the backend config.
* Use global variables on the backend config.


## Basic Usage

To generate a backend configuration you need to define a **backend** block,
very similar to how you would do on Terraform, but inside a
**terramate** block, like this:

```hcl
terramate {
  backend "type" {
    param = "value"
  }
}
```

And terramate will use that to generate Terraform code with a backend
configuration. A configuration can only provide one backend block
(overriding the config is possible, check
[Overriding Configuration](#overriding-configuration) for more details).

Let's start with a very simple example. Lets say your terramate project
has this layout:

```
.
└── envs
    ├── prod
    │   ├── stack-1
    │   └── stack-2
    └── staging
        ├── stack-1
        └── stack-2
```

You can define a prod backend configuration by creating the file
**envs/prod/terramate.tsk.hcl**:

```hcl
terramate {
  backend "type" {
    param = "prod"
  }
}
```

Then you can define a staging backend configuration by creating the file
**envs/staging/terramate.tsk.hcl**:

```hcl
terramate {
  backend "type" {
    param = "staging"
  }
}
```

And finally generate the final Terraform code on all the stacks of
your project, by running from the project top level directory:

```sh
terramate generate
```

Now you will see a **_gen_terramate.tsk.tf** file on each stack.
The files generated on the stacks inside **envs/prod** will be:

```hcl
terraform {
  backend "type" {
    param = "prod"
  }
}
```

The files generated on the stacks inside **envs/staging** will be:

```hcl
terraform {
  backend "type" {
    param = "staging"
  }
}
```

Any changes on the terramate backend configuration will require a generation
step to update the generated files. The generated files should never
be manipulated manually.


## Overriding Configuration

Every backend configuration should include just a single block/definition, but
this single definition can exist on any directory, providing a clear way to
override overall/general purpose configuration with more detailed/specific
configuration.

More specific configuration always override general purpose configuration.
There is no merge strategy/ composition involved, the configuration found
closest to a stack on the file system, or directly at the stack directory,
is the one used, ignoring any previous configuration.

As example, suppose we have this layout on a project:

```
.
└── envs
    ├── prod
    │   ├── stack-1
    │   └── stack-2
    └── staging
        ├── stack-1
        └── stack-2
```

If there is a backend configuration inside **envs**, it will be used
by all stacks. If a backend configuration is added on **envs/prod**,
now all stacks inside **envs/prod** will use this new configuration,
ignoring the one on **envs**, while **envs/staging** stacks will keep
using the general **envs** configuration.

Going further, if a backend configuration is added on **envs/prod/stack-1**,
that configuration will replace the one on **envs/prod** for **stack-1**, while
**envs/prod/stack-2** will continue to use the backend configuration defined
on **envs/prod**.


## Using metadata

Terramate provides a set of metadata information as documented [here](metadata.md).
Any metadata provided by terramate can be used on a backend configuration.

As a concrete example, given that we manage multiple stacks on GCP, it is useful
to define a backend configuration only once that uses specific stacks paths
as the prefix for the GCS storage, like this:

```hcl
terramate {
  backend "gcs" {
    bucket = "bucket-name"
    prefix = terramate.path
  }
}
```

Metadata is always evaluated on the context of a specific stack, so this
configuration can be added as an overall configuration for all stacks
and each stack will get a different configuration specific for it, given:

```
.
└── envs
    ├── prod
    │   ├── stack-1
    │   └── stack-2
    └── staging
        ├── stack-1
        └── stack-2
```

If the backend configuration mentioned above is created inside **envs**,
then the generated backend config for **envs/prod/stack-1** will be:

```
terraform {
  backend "gcs" {
    bucket = "bucket-name"
    prefix = "/envs/prod/stack-1"
  }
}
```

While for **envs/staging/stack-1** it will be:

```
terraform {
  backend "gcs" {
    bucket = "bucket-name"
    prefix = "/envs/staging/stack-1"
  }
}
```

And the same applies to all other stacks.


## Using globals

TODO: we don't have the globals spec yet, add it here once we got it.
