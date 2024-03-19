---
title: Code Generation
description: Learn how to keep your stacks DRY by generating files such as Terraform, Kubernetes Manifests and more.
---

# Code Generation

## Introduction

Terramate CLI supports the generation of code in stacks.
This section contains usage documentation and examples of the code generation in Terramate and all available strategies.

Generating code in stacks can help keep your stacks DRY (think generating files such as Terraform provider configuration
or Kubernetes manifests). This is especially useful whenever you want to conditionally generate code for stacks based on
stack metadata such as tags or paths.

Currently, the following code generation strategies are available:

* [HCL generation](./generate-hcl.md) with `stack` [context](#generation-context) to generate Terraform, OpenTofu and other HCL configurations.
* [File generation](./generate-file.md) with `root` and `stack` [context](#generation-context) to generate arbitrary
files such as JSON and YAML.

E.g., the following generates a simple file using the [file generation](./generate-file.md) strategy.

```hcl
# config.tm.hcl
generate_file "file.txt" {
  content = "file contents"
}
```

## Generate code

The [`generate`](../cmdline/generate.md) command generates all files configured by the available code generation strategies.

```sh
terramate generate
```

This command always runs the code generation in your entire Terramate project and ensures that all generated files
are up to date.

## Hierarchical code generation

Code generation can be defined and used anywhere in a [Terramate Project](../projects/configuration.md). 

Any stack that is part of the filesystem tree reachable from a code generation strategy configuration will be a selected target to generate code in.
This means a configuration to generate code can be defined at the root level, reach all stacks, and trigger code generation in all
stacks if not limited by [conditional code generation](#conditional-code-generation) or
[stack filters](./generate-hcl.md#filter-based-code-generation).

### Failure modes

There is no overriding or merging behavior for code generation blocks.
Blocks defined at different levels with the same label aren't allowed, resulting
in failure for the overall code generation process.

## Import

Terramate configuration files that define code generation strategies can also be imported using the `import` block.

```hcl
import {
  source = "/modules/generate_providers.tm.hcl"
}
```

For details, please see the [import documentation](../configuration/index.md#import-block-schema).

## Generation context

Code generation supports two execution contexts:

- **stack**: generates code relative to the stack where it's defined.
- **root**: generates code outside of stacks.

The `stack` context gives access to all code generation features, such as:

* [Globals](./variables/globals.md)
* [All Metadata](./variables/metadata.md)
* [Functions](./functions/index.md)
* [Lets](./variables/lets.md)
* [Assertions](#assertions)

But the `root` context gives access to:

* [Project Metadata](./variables/metadata.md#project-metadata)
* [Functions](./functions/index.md)
* [Lets](./variables/lets.md)

If not specified the default generation context is `stack`.
The `generate_hcl` block doesn't support changing the `context`, it will always be
of type `stack`. The `generate_file` block supports the `context` attribute which you can explicitly change to `root`.

**Example:**

```hcl
generate_file "/file.txt" {
  context = root
  content = "something"
}
```

## Labels

All code generation blocks use labels to identify the block and define where
the generated code will be saved but they have different constraints depending
on the [generation context](#generation-context).

For `stack` context, the labels must follow the constraints below:

* It is a relative path in the form `<dir>/<filename>` or just `<filename>`
* It is always defined with `/` independent on the OS you are working on
* It does not contain `../` (code can only be generated inside the stack)
* It does not start with `./`
* It does not contain a dot directory (invalid example: `somedir/.invalid/main.tf`)
* It is not a symbolic link
* It is not a stack
* It is unique on the whole hierarchy of a stack for all blocks with condition=true.

For `root` context, the constraints are:

* It is an absolute path in the form `/<dir>/<filename>` or just `/<filename>`.
* It is always defined with `/` independent on the OS you are working on
* It does not contain `../` (code can only be generated inside the project root)
* It does not contain a dot directory (invalid example: `somedir/.invalid/file.txt`)
* It is not a symbolic link
* It is not a stack
* It is unique on the whole hierarchy for all blocks with condition=true.

**Example:**

```hcl
# The label defines that the code generation will
# generate a file named '_generate_main.tf'
generate_hcl "_generate_main.tf" {
  content {
    resource "aws_vpc" "main" {
      cidr_block = "10.0.0.0/16"
    }
  }
}
```

## Conditional code generation

Terramate offers different ways to generate code conditionally.

### Stack filters - path-based code generation

Stack Filters can be used to trigger the generation of code only within matching stacks identified by the stack absolute path.
Globbing support in the path filters allows pattern matching with the hierarchy, thus targeting stacks in directories with
specific names or following a specific pattern. Stack filters will be extended to support more matching strategies on other
metadata.

Stack Filters allow path-based code generation on the stack path within the repository or within a project. They are
defined using `generate_hcl.stack_filter` blocks and are explained in the
[Generate HCL](./generate-hcl.md) and [Generate File](./generate-file.md) sections.
They can be combined with conditions and are evaluated before conditions.

**Example:**

```hcl
generate_hcl "vpc.tf" {
  stack_filter {
    project_paths = [
      "/path/to/specific/stack", # match exact path
      "/path/to/some/stacks/*",  # match stacks in a directory
      "/path/to/many/stacks/**", # match all stacks within a tree
    ]
  }
  
  content {
    resource "aws_vpc" "main" {
      cidr_block = "10.0.0.0/16"
    }
  }
}
```

### Conditions

Conditions allow for more complex targeting of stacks using Terramate Globals, Terramate Metadata and Terramate Functions.
As complex calculations require more CPU time and access more data, the execution of conditions is significantly slower compared to using Stack Filters.

The following list names some examples of what conditions could be used for:
- Path-based Code Generation (prefer Stack Filters or use in combination with Stack Filters for large amounts of stacks)
- Tag-based Code Generation
- Generation of code based on the state of Terramate Globals
- [Metadata](./variables/metadata.md)-based Code Generation when using the `terramate` variable namespace to access more stack details.
- Any combination of the above
- Any complex calculation using Terramate Functions or Terramate Variables

**Example:**

```hcl
# Generate Terraform backend configuration
# for stacks that are tagged with "terraform"
generate_hcl "_terramate_generated_backend.tf" {
  condition = tm_contains(terramate.stack.tags, "terraform")

  content {
    terraform {
      backend "s3" {
        region         = global.terraform.backend.s3.region
        bucket         = global.terraform.backend.s3.bucket
        key            = "terraform/stacks/by-id/${terramate.stack.id}/terraform.tfstate"
        encrypt        = true
        dynamodb_table = "terraform_state"
      }
    }
  }
}
```

## Assertions

Assertions can be used to fail code generation for one or more stacks
if some pre-condition is not met, helping to catch mistakes in your configuration.

Assertions can be only used when the [generation context](#generation-context)
is of type `stack` and it has the following fields:

- `assertion` *(optional boolean)* The `condition` attribute supports any expression that renders to a boolean.
- `message` *(optional string)* Obligatory, must evaluate to string
- `warning` *(optional string)*  Must evaluate to boolean. Defaults to `false`.

All fields can contain expressions using Terramate Variables (`let`, `global`, and `terramate` namespaces) and all Terramate Functions are supported.

```hcl
assert {
  assertion = global.a == global.b
  message   = "assertion failed, details: ${global.a} != ${global.b}"
}
```

When the **assertion** is `false` in the context of a stack, code generation for
that stack will fail and the reported error will be the one provided on the
**message** field. The stack won't be touched, no files will be changed/created/deleted.

Optionally the **warning** field can be defined and if it is evaluated to `true`
then a false **assertion** will **not** generate an error. Code will be generated,
but a warning output will be shown during code generation.

The **assert** block has hierarchical behavior, any assert blocks defined in a
directory will be applied to all stacks inside this directory. For example, an
**assert** block defined on the root of a project will be applied to all stacks
in the project.

Assert blocks can also be defined inside `generate_hcl` and `generate_file` blocks.
When inside one of those blocks it has the same semantics as described above, except
that it will have access to locally scoped data like the `let` namespace.

## Best practices

We recommend the following best practices when working with code generation.

### Add generated files to your Git repository

Commit the generated files to the repository. This can be achieved either by manually running `terramate generate`
locally or by automating `terramate generate` in your CI/CD pipelines.
In any case, the generated files should be available to the Pull Request reviewer so they can spot misconfigurations.

### Use a prefix for generated files

We recommend prefixing generated files with `_terramate_generated_` (e.g., `_terramate_generated_main.tf`) or similar to be
able to easily separate generated files from non-generated files.

In addition to the recommended prefix, each generated file contains a header to indicate that if users modify the generated files
all changes will be overwritten by the `terramate generate`.

Example:
```hcl
// TERRAMATE: GENERATED AUTOMATICALLY DO NOT EDIT
....
```
