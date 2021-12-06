# Backend Configuration

Terrastack provides some facilities to improve on how backend configuration
is managed on terraform. The idea is to circumvent some of the limitations
from terraform that makes it really hard to avoid duplication/mistakes
when managing backend configuration.

There is no way to define a single parametrized backend configuration
that can then be re-used across different stacks/environments/etc.

You can't even use a local variable as a parameter on the backend config,
from [terraform docs](https://www.terraform.io/docs/language/settings/backends/configuration.html):

```
A backend block cannot refer to named values
(like input variables, locals, or data source attributes).
```

With those limitations in mind, terrastack provides a way to:

* Define a single parametrized backend config and re-use it on multiple stacks.
* Use terrastack metadata, like stack name/path, on the backend config.
* Use global variables on the backend config.


## Basic Usage

To generate a backend configuration you need to define a **backend** block,
very similarly to how you would do on terraform, but inside a
**terrastack** block, like this:

```hcl
terrastack {
  backend "type" {
    param = "value"
  }
}
```

And terrastack will use that to generate terraform code with a backend
configuration. A configuration can only provide one backend block
(overriding the config is possible, check
[Overriding Configuration](#overriding-configuration) for more details).

Let's start with a very simple example. Lets say your terrastack project
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
**envs/prod/terrastack.tsk.hcl**:

```hcl
terrastack {
  backend "type" {
    param = "prod"
  }
}
```

Then you can define a staging backend configuration by creating the file
**envs/staging/terrastack.tsk.hcl**:

```hcl
terrastack {
  backend "type" {
    param = "staging"
  }
}
```

And finally generate the final terraform code on all the stacks of
your project, by running from the project top level directory:

```sh
terrastack generate
```

Now you will see a **_gen_terrastack.tsk.tf** file on each stack.
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

Any changes on the terrastack backend configuration will require a generation
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

## Using globals
