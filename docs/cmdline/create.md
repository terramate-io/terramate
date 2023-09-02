---
title: terramate create - Command
description: With the terramate create command you can create a new stack in the current project.

prev:
  text: 'Cloud Info'
  link: '/cmdline/cloud-info'

next:
  text: 'Eval'
  link: '/cmdline/eval'
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

Initialize Terramate in an existing Terraform project:

```bash
terramate create --all-terraform
```

This is helpful when you want to onboard Terramate in an existing
Terraform project. `--all-terraform` will create a Terramate configuration
file in every Terraform directory that contain a `terraform.backend` block or `provider` blocks.


## Options

- `--id=STRING` ID of the stack. Defaults to a random UUIDv4 (Using the default is highly recommended).
- `--name=STRING` Name of the stack. Defaults to stack dir base name.
- `--description=STRING` Description of the stack. Defaults to the stack name.
- `--tags=LIST` Adds tags to the stack. Example: `--tags a --tags b` or `--tags a,b`.
- `--import=LIST`  Add import block for the given path on the stack. Example: `--import dir/path1 --import dir/path2` or `--import dir/path1,dir/path2`
- `--after=LIST`, `--before=LIST` Define an explicit [order of execution](../orchestration/index.md#explicit-order-of-execution). LIST can contain relative paths `../path/to/stack` and/or tags `tag:my-tag`. These options can be used multiple times.
- `--ignore-existing` If the stack already exists do nothing and don't fail.
- `--all-terraform` Initialize Terramate in all directories containing `terraform.backend` blocks.
- `--ensure-stack-ids` Ensures that every stack has an UUID.
- `--no-generate` Disable code generation for the newly created stack.
