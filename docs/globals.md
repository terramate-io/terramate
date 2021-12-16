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

```
terramate {
  // other Terramate related configs
}

globals {
  env = "staging"
  values = [ 1, 2 , 3 ]
}
```

And you can reference them on Terramate configuration:

```
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
they are referenced, the idea is to be able to define
core/base configurations and use them across the entire project.

Globals are evaluated on the context of a stack, evaluation starts
with any globals defined on the stack itself and then keeps going up
on the file system until the project root is reached.

If global variables don't have the same name, then globals are just merged
together as they are evaluated going up on the project file system.

If global variables have the same name, the most specific global overrides
the more general one, where by specific we mean the global closes to the
stack being evaluated.



## Referencing globals on terraform code

TODO
