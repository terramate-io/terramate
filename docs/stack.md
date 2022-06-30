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

Stack configurations related to orchestration can be found [here](orchestration.md).

Besides orchestration the `stack` block also define attributes that are
used to describe the `stack`.

## stack.id (string)(optional)

The stack ID **must** be a string composed of alphanumeric chars + `-` + `_`.
The ID can't be bigger than 64 bytes. The ID **must** be unique on the
whole project.

There is no default value determined for the stack ID.

Eg:

```hcl
stack {
  id = "some_id_that_must_be_unique"
}
```

## stack.name (string)(optional)

The stack name can be any string and it defaults to the stack directory
base name.

Eg:

```hcl
stack {
  name = "My Awesome Stack Name"
}
```

## stack.description (string)(optional)

The stack description can be any string and it defaults to an empty string.

Eg:

```hcl
stack {
  description = "My Awesome Stack Description"
}
```
