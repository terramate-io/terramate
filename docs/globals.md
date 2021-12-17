# Global Configuration

Globals provide a way to define information that can be re-used
across stacks with a clear hierarchical/merge semantic.

Here we explain the overall semantics on how globals are defined
inside a Terramate project, they can be used in different ways
inside stacks and for specific details check the documentation
of each feature that leverages globals: 

* [backend configuration](backend-config.md)

Defining globals is fairly straightforward, you just need to
add a **globals** block to your [Terramate configuration file](config.md):

```hcl
terramate {
  // other Terramate related configs
}

globals {
  env = "staging"
  values = [ 1, 2, 3 ]
}
```

And you can reference them on Terramate configuration:

```hcl
terramate {
  backend "type" {
    param = "value-${global.env}"
  }
}

globals {
  env = "staging"
}
```

Globals don't need to be defined on the same configuration
they are referenced, users can define
core/base configurations and use them across the entire project.

Globals are evaluated on the context of a stack, evaluation starts
with any globals defined on the stack itself and then keeps going up
on the file system until the project root is reached.

If global variables don't have the same name, then globals are just merged
together as they are evaluated going up on the project file system.

If global variables have the same name, the most specific global overrides
the more general one, where by specific we mean the global closest to the
stack being evaluated.

Given a project structured like this:

```
.
└── stacks
    ├── stack-1
    │   └── terramate.tm.hcl
    └── stack-2
        └── terramate.tm.hcl
```

The global evaluation order for stack-1, from higher to lower precedence, is:

* stacks/stack-1
* stacks
* . (the project root dir)

To create globals for the entire project just add a
[Terramate configuration file](config.md) on the project
root with some useful globals:

```hcl
globals {
  project_name = "awesome-project"
  useful       = "useful"
}
```

Now any stack on the project can reference these globals on their
Terramate configuration, like this backend config example:

```hcl
terramate {
  backend "type" {
    param = "${global.project_name}-${global.useful}"
  }
}
```

Now lets say one of the stacks wants to add more globals, to do
so we can add globals on the stack configuration file
**stacks/stack-1/terramate.tm.hcl**:

```hcl
terramate {
  // ommited
}

stack {
  // ommited
}

globals {
  stack_data = "some specialized stack-1 data"
}
```

Now the globals available to **stacks/stack-1** are:

```
project_name = "awesome-project"
useful       = "useful"
stack_data   = "some specialized stack-1 data"
```

And the globals available to **stacks/stack-2** :

```
project_name = "awesome-project"
useful       = "useful"
```

Overall **stacks/stack-1** is getting a full merge of all
its globals + all globals defined on each dir until reaching
the project root.

Now lets say **stacks/stack-1** needs to override one of the globals,
we just redefine the global on **stacks/stack-1/terramate.tm.hcl**:

```hcl
terramate {
  // ommited
}

stack {
  // ommited
}

globals {
  useful     = "overriden by stack-1"
  stack_data = "some specialized stack-1 data"
}
```

Now the globals available to **stacks/stack-1** are:

```
project_name = "awesome-project"
useful       = "overriden by stack-1"
stack_data   = "some specialized stack-1 data"
```

And the globals available to **stacks/stack-2** remains:

```
project_name = "awesome-project"
useful       = "useful"
```

Overriding happens at the global name level, so objects/maps/lists/sets
won't get merged, they are completely overwritten by the most
specific configuration with the same global name.

Lets say we add this to our project wide configuration:

```hcl
globals {
  project_name = "awesome-project"
  useful       = "useful"
  object       = { field_a = "field_a", field_b = "field_b" }
}
```

And override it on **stacks/stack-1/terramate.tm.hcl**:

```hcl
terramate {
  // ommited
}

stack {
  // ommited
}

globals {
  object = { field_a = "overriden_field_a" }
}
```

The globals available to **stacks/stack-1** will be:

```
project_name = "awesome-project"
useful       = "useful"
object       = { field_a = "overriden_field_a" }
```

And the globals available to **stacks/stack-2**:

```
project_name = "awesome-project"
useful       = "useful"
object       = { field_a = "field_a", field_b = "field_b" }
```


## Referencing globals on terraform code

TODO
