---
title: terramate create - Command
description: With the 'terramate create' command you can create a new stack or import existing Terraform or Terragrunt configurations.
---

# Create or Import Stacks

The `terramate create` command creates a new stack in a given directory aka stack-path.
If the stack-path does not yet exist, it will be created.
Code Generation will be triggered by default to initialize the newly created stack.

Stack metadata can be set upon creation of the stack.

This metadata will also be synchronized to Terramate Cloud if enabled and can help you and your team to browse through stacks better.

**It can also be used to import existing Terraform or Terragrunt configurations and initialize stacks automatically for an easy on-boarding.**

# Examples

## Create a new stack

This is the most basic example to create a stack in a directory called `path/to/stack`.

```bash
terramate create path/to/stack
```

## Import existing Terraform Root Modules

If you already have an existing Terraform setup in your repository and want to initialize all directories containing Terraform configurations as Terramate Stacks, you can use `--all-terraform` option to automatically detect those stacks and initialize them.

Terramate detects all Terraform directories that contain a `backend {}` configuration as a stack for you.

```bash
terramate create --all-terraform
```

## Import existing Terragrunt Modules

Similar to detecting Terraform directories, existing Terragrunt configurations can be easily initialized as Terramate Stacks.

Terramate detects all Terragrunt Modules that contain a `terraform.source` configuration as a stack for you.

```bash
terramate create --all-terragrunt
```

If the Terragrunt module declare dependencies then the created stack will have its ordering
attributes automatically set.

Terramate also has experimental support for Terragrunt in the change detection, check the [Terragrunt integration](../change-detection/integrations/terragrunt.md) page for more information.

# Usage

`terramate create [options] <stack-path>`

`terramate create --all-terraform`

`terramate create --all-terragrunt`

`terramate create --ensure-stack-ids`

In the basic use case `[options]` can be one or multiple of:

## Set the ID

- `--id <id>`

  Default: A new UUIDv4.

  Stack IDs need to be unique within the repository.
  It is recommended to use an UUID as the stack id.

  Example:

  ```bash
  terramate create --id "my-id" path/to/stack
  ```

## Set the name

- `--name <name>`

  Default: The basename of the `<stack-path>`.

  The stack name should help users to understand what a stack is containing.

  It will be used in Terramate Cloud when listing stacks in addition to the `<stack-path>`.

  Example:

  ```bash
  terramate create --name "My first Terramate stack" path/to/stack
  ```

## Set the description

- `--description <description>`

  Default: the `<name>` of the stack.

  The description should give more details and context about the stack and the resources managed.

  It will be visible in Terramate Cloud when showing stack details and can help your team to understand previews, deployments and drift details.

  Example:

  ```bash
  terramate create --description "This stack contains amazing resources for my service" path/to/stack
  ```

## Set tags

- `--tags <tags>` A comma separated list of tags to add to the stack.

  This option can be used multiple times to define additional tags.

  Tags can be used to filter stacks when listing them or executing commands in them.
  They can also be used to define the order of execution with `before` and `after` configurations.

  Example:

  ```bash
  terramate create --tags "mytag,yourtag,gutentag" path/to/stack-a
  terramate create --tags "mytag" --tag "yourtag" --tag "gutentag" path/to/stack-b
  ```

## Add imports

- `--import <path>` A comma separated list of directories or files to add import blocks for in the stacks configuration.

  This option can be used multiple times to define additional import blocks.

  Example:

  ```bash
  terramate create --import "path/to/directory" --import "path/to/file" path/to/stack
  ```

## Add explicit order of execution

- `--after <list>` A comma separated list of filters.

  This option can be used multiple times to define additional `after` relationships.

  The `<list>` can contain a mix of

  - tags e.g. `tag:my-tag` _(recommended)_
  - relative paths: e.g `../path/to/stack` to stacks (not recommended)

  When defined, this stack will be executed after matching stacks.

  It is recommended to use tags to maintain refactoring of stack hierarchy.
  Using stack-paths is available but not recommended to use anymore.
  When using nested stacks, an implicit order of execution is defined for you.

  Example:

  ```bash
  terramate create --after "tag:my-tag" --after "../another-stack" path/to/stack
  ```

- `--before <list>` A comma separated list of filters.

  This option can be used multiple times to define additional `before` relationships.

  The `<list>` can contain a mix of

  - tags e.g. `tag:my-tag` _(recommended)_
  - relative paths: e.g `../path/to/stack` to stacks

  When defined, this stack will be executed before matching stacks.

  It is recommended to use tags to maintain refactoring of stack hierarchy.
  Using stack-paths is available but not recommended to use anymore.
  When using nested stacks, an implicit order of execution is defined for you.

  Example:

  ```bash
  terramate create --before "tag:my-tag" --before "../another-stack" path/to/stack
  ```

## Additional options

- `--ignore-existing`

  If set, and the stack already exists do nothing and don't fail.

  Example:

  ```bash
  terramate create path/to/stack
  terramate create --ignore-existing path/to/stack
  ```

- `--no-generate`

  Disable code generation for the newly created stacks

## Special use cases

- `terramate create --all-terraform`

  Detect and initialize all Terraform directories detected.
  See example above.

- `terramate create --all-terragrunt`

  Detect and initialize all Terragrunt directories detected.
  See example above.

- `terramate create --ensure-stack-ids`

  Ensures that every stack has an ID set.
  Stacks that do not have an `id` defined, will get a new UUIDv4 generated and set.

  A stack ID for every stack is required in order to synchronize stacks to Terramate Cloud.

[order of execution]: ../orchestration/index.md#explicit-order-of-execution
