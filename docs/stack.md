# Stack

When working with Infrastructure as Code it's considered to be a best practice
to split up and organize your IaC into several smaller and isolated stacks.

Typically, each stack comes with its own Terraform state which allows us
to plan and apply each stack on its own.

A Terramate stack is:

* A directory inside your project.
* Has at least one or more Terramate configuration files.
* One of the configuration files has a `stack {}` block on it.
* It has no stacks on any of its subdirs (stacks can't have stacks inside them).

What separates a stack from any other directory is the `stack{}` block.
It doesn't require any attributes by default, but it can be used
to describe stacks and orchestrate their execution.

You can change a stack name and provide a description to it by using
the attributes `name` and `description`:

```hcl
stack {
  name        = "My Awesome Stack"
  description = "Such Awesome Much Stack"
}
```

Details on further stack configurations related to orchestration
can be found [here](orchestration.md).
