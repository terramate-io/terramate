# Code Generation Configuration

Terramate provides facilities to control how code generation happens
inside a project. Allowing you to configure code generation for
a single stack or for all stacks in a project, leveraging the
project hierarchy.

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

Example changing both backend and locals generated code filenames:

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

Let's start with a very simple example. Lets say your Terramate project
has this layout:

```
.
└── stacks
    ├── stack-1
    └── stack-2
```

You can change how code is generated for **stacks/stack-1** by changing
the stack configuration **stacks/stack-1/terramate.tm.hcl** :

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
