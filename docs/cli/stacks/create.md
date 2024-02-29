---
title: Create Stacks
description: Learn how to create and manage Infrastructure as Code agnostic stacks with Terramate.
---

# Create stacks

## Create a stack

Stacks can be created using the [create](../cmdline/create.md) command, e.g.:

```sh
terramate create [options] <directory>
```

By default, this will create a directory with a `stack.tm.hcl` file that contains the configuration for your stack,
which comes with some default properties:

```sh
terramate create stacks/vpc --name "Main VPC" --description "Stack to manage the main VPC"
```

```hcl
# ./stacks/vpc/stack.tm.hcl
stack {
  name        = "My first stack"
  description = "Stack to manage the main VPC"
  id          = "3271f37c-0e08-4b59-b205-1ee61082ff26"
}
```

- `name`: name of the stack, defaults to the basename of the directory
- `description`: description of the stack, defaults to the basename of the directory
- `id`: a project-wide random UUID

You can use all available configuration properties as attributes to the [create](../cmdline/create.md) command, e.g.
For an overview of all properties available for configuring stacks,
please see [stacks configuration](./configuration.md) documentation.

## Generating code when creating stacks

Whenever you create a new stack using the [create](../cmdline/create.md) command, Terramate will automatically run the
code generation for you. This comes especially handy when you want to automatically create files such as
Terraform configurations for each newly created stack.

For example, the following Terramate configuration will automatically create the Terraform backend configuration using the
`stack.id` property as a reference for the key of the Terraform state in all stacks that are tagged with `terraform`.

```hcl
generate_hcl "_terramate_generated_backend.tf" {
  condition = tm_contains(terramate.stack.tags, "terraform")

  content {
    terraform {
      backend "s3" {
        region         = "us-east-1"
        bucket         = "terraform-state-bucket"
        key            = "terraform/stacks/by-id/${terramate.stack.id}/terraform.tfstate"
        encrypt        = true
        dynamodb_table = "terraform_state"
      }
    }
  }
}
```

Running the code generation upon stack creation can be disabled by passing the `-no-generate` argument to the
[create](../cmdline/create.md) command, e.g.:

```hcl
terramate create -no-generate <path>
```

For details, please see [Code Generation](../code-generation/index.md) documentation.
