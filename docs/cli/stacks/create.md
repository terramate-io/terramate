---
title: Create Stacks
description: Learn how to create and manage Infrastructure as Code agnostic stacks with Terramate.
---

# Create Stacks

## Create a plain stack

Stacks can be created using the [terramate create](../cmdline/create.md) command.
The command accepts a set of options to allow to initialize all its details programmatically.
You can set metadata like the `name`, `description`, add `tags`, or define an order of execution.

```sh
terramate create <directory>
```

By default, the `terramate create` command will create a directory and add a `stack.tm.hcl` file that contains the configuration for your stack adding the directory basename as `name` and `description` and creating a UUIDv4 as `id` that needs to be unique within the current reposiory and identifies the stack in Terramate Cloud if connected.

It is recommended to never change the `id` once committed to allow tracking of the stack when refactoring the directory hierarchy.

The generated file is an HCL file and can be edited and extended at any point in time.

The following example

```sh
terramate create stacks/vpc --name "Main VPC" --description "Stack to manage the main VPC"
```

will lead to the creation of the following file:

```hcl
# ./stacks/vpc/stack.tm.hcl
stack {
  name        = "My first stack"
  description = "Stack to manage the main VPC"
  id          = "3271f37c-0e08-4b59-b205-1ee61082ff26"
}
```

You can use all available configuration properties as attributes to the [terramate create](../cmdline/create.md) command, e.g.

For an overview of all properties available for configuring stacks,
please see [Stack Configuration](./configuration.md) documentation.

Terramate detects stacks based on the exitance of a `stack {}` Block. The name of the file is not important and can be different from `stack.tm.hcl`. There can be exactly one stack block defined in a stack.

For new stacks, [Code Generation](../code-generation/index.md) will be triggered so that the new stack gets initialized with a default configuration if desired.

## Import existing stacks

Terramate can detect and import various existing configurations.

- `terramate create --all-terraform` will [import existing Terraform](../on-boarding/terraform.md)
- `terramate create --all-terragrunt` will [import existing Terragrunt](../on-boarding/terragrunt.md)
