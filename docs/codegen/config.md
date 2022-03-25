# Code Generation Configuration

Terramate provides facilities to control how code generation happens
inside a project. Allowing you to easily configure code generation for
a single stack or for all stacks in a project.

## Basic Usage

To control code generation you need to define a **terramate.config.generate**
block on your Terramate configuration. Like this:

```hcl
terramate {
  config {
    generate {
        # Code Generation Related Configs
    }
  }
}
```

Where the available configurations are:

* `backend_config_filename` : filename of generated backend config code
* `locals_filename` : filename of generated locals code


Let's start with a simple example. Lets say your Terramate project
has this layout:

```
.
└── stacks
    ├── stack-1
    └── stack-2
```

You can change how code is generated for **stacks/stack-1** by changing
the stack configuration file **stacks/stack-1/terramate.tm.hcl** :

```hcl
terramate {
  config {
    generate {
      backend_config_filename = "backend.tf"
      locals_filename = "locals.tf"
    }
  }
}

stack {}
```

Now when you run `terramate generate`, while **stacks/stack-2** will have
default filenames for the generated code, for **stacks/stack-1** you will get:

```
stacks/stack-1/backend.tf
stacks/stack-1/locals.tf
```

Lets say now we want the same setup for both stacks, instead of duplicating the
configuration, we can just create a configuration file on the parent dir of both
stacks **stacks/terramate.tm.hcl** :

```hcl
terramate {
  config {
    generate {
      backend_config_filename = "backend.tf"
      locals_filename = "locals.tf"
    }
  }
}
```

Now both stacks, or any new stack added inside **stacks**, will generate code
using this configuration. Since the configuration can be defined on any level of
the hierarchy of the project file system, it does raise the question of how
the overriding of the configuration files behaves.

More specific configuration always override general purpose configuration.
There is no merge strategy or composition involved, the configuration found
closest to a stack on the file system, or directly at the stack directory,
is the one used, ignoring any configuration on parent directories.

Lets take as an example the previous configuration on **stacks/terramate.tm.hcl** :

```hcl
terramate {
  config {
    generate {
      backend_config_filename = "backend.tf"
      locals_filename = "locals.tf"
    }
  }
}
```

Lets say that for **stacks/stack-1** you want a different configuration.
To do that you just need to add a configuration file on
the stack itself and it will override the one from the parent directory.

This works at any level of the hierarchy, so you can organize configurations
in a way that you have sensible defaults but can override them for specific
stacks or subset of stacks, depending on how your project is organized.

It is invalid to define the generate block more than once in the same
configuration file. This is invalid for example:


```hcl
terramate {
  config {
    generate {
      backend_config_filename = "backend.tf"
    }
    generate {
      locals_filename = "locals.tf"
    }
  }
}
```
