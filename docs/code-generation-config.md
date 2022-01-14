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
