---
title: terramate create - Command
description: With the terramate create command you can create a new stack in the current project.
---

# Create

The `create` command creates a new stack. This command accepts a path that will
be used to create the stack. The `create` command will create intermediate
directories as required. Whenever the `create` command is used, the code generation will run automatically to ensure all code relevant for the newly created stack will be generated.

## Usage

`terramate create [options] PATH`

## Examples

Create a new stack:

```bash
terramate create path/to/stack
```

Complete Example:

```bash
terramate create path/to/stack \
    --name foo-stack \
    --description "This is an example" \
    --tags app,prd \
    --after ../../after-another-stack,../../after-this-stack \
    --before ../../before-this-stack,../../before-another-stack \
    --ignore-existing \
    --no-generate
```

The `terramate create` supports initializing Terramate stacks in an existing [Terraform](https://www.terraform.io/)
or [Terragrunt](https://terragrunt.gruntwork.io/) project.

Scan and create stacks for Terraform stacks:

```bash
terramate create --all-terraform
```

Scan and create stacks for Terragrunt stacks/modules:

```bash
terramate create --all-terragrunt
```

This is helpful when you want to onboard Terramate in an existing project. 

The `--all-terraform` will create a Terramate stack file in every Terraform directory that 
contains a `terraform.backend` block or `provider` blocks.

The `--all-terragrunt` will create a Terramate stack file in every Terragrunt configuration that
defines a `terraform.source` attribute (it will correctly parse and include other files as needed).

## Options

- `--id=STRING` ID of the stack. Defaults to a random UUIDv4 (Using the default is highly recommended).
- `--name=STRING` Name of the stack. Defaults to stack dir base name.
- `--description=STRING` Description of the stack. Defaults to the stack name.
- `--tags=LIST` Adds tags to the stack. Example: `--tags a --tags b` or `--tags a,b`.
- `--import=LIST` Add import block for the given path on the stack. Example: `--import dir/path1 --import dir/path2` or `--import dir/path1,dir/path2`
- `--after=LIST`, `--before=LIST` Define an explicit [order of execution](../orchestration/index.md#explicit-order-of-execution). LIST can contain relative paths `../path/to/stack` and/or tags `tag:my-tag`. These options can be used multiple times.
- `--ignore-existing` If the stack already exists do nothing and don't fail.
- `--all-terraform` Initialize Terramate in all directories containing `terraform.backend` blocks.
- `--ensure-stack-ids` Ensures that every stack has a UUID.
- `--no-generate` Disable code generation for the newly created stack.
