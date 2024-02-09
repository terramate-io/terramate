---
title: Configure Terramate
description: Learn about the different configuration options in Terramate ranging from avoiding duplication by leveraging powerful code generation to flexible orchestration.
---

# Terramate Configuration Overview

Different configurations can be done in Terramate, ranging from avoiding
duplication by leveraging powerful code generation to flexible orchestration by
allowing control of stacks order of execution.

To do so, Terramate works with configuration files that have the suffixes:

* `tm.hcl`
* `tm`

## Configuration files

Terramate files can be found in any non-hidden directory of a Terramate project
and all non-hidden files in a single directory will be handled as the
concatenation of all of them in a single file, forming a single **configuration**.

The configuration blocks can be defined multiple times and their values are merged
whenever possible. See [Config Merging](#config-merging) for details.

## Importing configurations

Each configuration can import other configurations using the `import` block.
See the example below:

```hcl
# globals.tm.hcl

import {
    # import a specific file
    source = "/more/globals.tm.hcl"
}
```

The import block supports globs as well:

```hcl
# globals.tm.hcl

import {
    # import all files in a directory
    source = "/imports/*.tm.hcl"
}
```

The `source` must reference a file using a relative path or an absolute path
relative to the project's root. Only files inside the project can be imported
and they must be from disjoint directories, which means you cannot import files
from parent directories as they're already visible in the child configuration.

The *imported* file is handled as if it's in the directory of the *importing*
file, then the same [merging strategy](#config-merging) applies for the case of
duplicated blocks being defined.

The `import` block do not support [merging](#config-merging) of its attributes
and multiple blocks can be defined in the same file or directory given that their
`source` attributes are different. In other words, each file can only be imported
once into a single configuration set.

An imported file can import other files but cycles are not allowed.

## Terramate Projects

A Terramate project is essentially a collection of Terraform code organized into
stacks.

It is not a hard requirement for Terramate to work that the project uses Git
for version control (support to other VCS might be added in the future), but
features like change detection do depend on a VCS to work and will fail if this
soft requirement is not met.

In general, a Terramate project looks like this:

* A Git project.
* The git top-level dir is the project root dir.
* Stacks are organized as different directories.
* Configuration may be present on any directory.

# Config merging

Multiple configuration blocks of the same type defined in a directory are merged into
a single configuration, provided their contents do not conflict. For instance, the
block definitions can be split into several blocks, with each one defining a part of
the whole. The exceptions to this are the [generate](../code-generation/index.md) and
`import` blocks.

The [globals](../code-generation/variables/globals.md) block extends the merging to the hierarchy of globals.

For example, the configuration below is valid:

```hcl
terramate {
    required_version = "~> 0.1"
}

terramate {
    config {
        git {
            default_branch = "main"
        }
    }
}
```

And the blocks can also be defined in different files.

But the following is invalid:

```hcl
terramate {
    required_version = "~> 0.1"
}

terramate {
    required_version = "~> 0.2"
}
```

# Skipping Directories

Terramate provides an option to ignore a non-hidden directory by creating an empty
file named `.tmskip` inside the directory. Terramate features will ignore this
directory and its contents, even if it contains Terramate files.

However, code located inside such a directory can still be imported.

# Terramate Configuration Schema

The terramate configuration is defined by the following top-level blocks:

- [terramate](#terramate-block-schema)
- [stack](#stack-block-schema)
- [globals](#globals-block-schema)
- [generate_file](#generate_file-block-schema)
- [generate_hcl](#generate_hcl-block-schema)
- [import](#import-block-schema)
- [vendor](#vendor-block-schema)

## terramate block schema

For detailed information about this block, see the [Project Configuration](../projects/configuration.md#project-configuration) docs.

The `terramate` block has no labels, supports [merging](#config-merging) and has
the following schema:

| name             |      type      | description |
|------------------|----------------|-------------|
| required_version |     string     | [version constraint](https://www.terraform.io/language/expressions/version-constraints) |
| required_version_allow_prereleases | bool | Enable _prerelease_ matches in the `required_version` constraint. |
| [config](#terramateconfig-block-schema) |     block      | project configuration |

## terramate.config block schema

The `terramate.config` block has no labels and has the following schema:

| name             |      type      | description |
|------------------|----------------|-------------|
| [git](#terramateconfiggit-block-schema) | block | git configuration |
| disable_safeguards | set(string) | list of safeguards to be disabled |

## terramate.config.git block schema

The `terramate.config.git` block has no labels and has the following schema:

| name             |      type      | description | default |
|------------------|----------------|-------------|---------|
| default\_branch | string | The default git branch |
| default\_remote | string | The default git remote |
| default\_branch\_base\_ref| string | The default git branch base reference |
| check\_untracked | boolean | (DEPRECATED) Enable check of untracked files | true
| check\_uncommitted | boolean | (DEPRECATED) Enable check of uncommitted files | true
| check\_remote | boolean | (DEPRECATED) Enable checking if local main is updated with remote | true

## terramate.config.generate block schema

The `terramate.config.generate` block has no labels and has the following schema:

| name             |      type      | description | default |
|------------------|----------------|-------------|---------|
| hcl\_magic\_header\_comment\_style | string | The comment style used in `generate_hcl`` blocks | "//"

## terramate.config.run block schema

The `terramate.config.run` block has no labels and has the following schema:

| name             |      type      | description | default |
|------------------|----------------|-------------|---------|
| check\_gen\_code | boolean | (DEPRECATED) Enable check for up to date generated code | true

## terramate.config.run.env block schema

The `terramate.config.run.env` block has no labels and it allows arbitrary
attributes. Each attribute **must** evaluate to a string.

More details can be found [here](../projects/configuration.md#the-terramateconfigrunenv-block).

## stack block schema

The `stack` block has no labels, **does not** support [merging](#config-merging)
and has the following schema:

| name             |      type      | description |
|------------------|----------------|-------------|
| id               | string         | The id of the stack |
| name             | string         | The name of the stack |
| description      | string         | The description of the stack |
| tags             | list(string)   | The tags of the stack |
| before           | list(string)   | The list of `before` stacks. See [ordering](../orchestration/index.md#stacks-ordering) docs. |
| after            | list(string)   | The list of `after` stacks. See [ordering](../orchestration/index.md#stacks-ordering) docs |
| wants            | list(string)   | The list of `wanted` stacks. See [ordering](../orchestration/index.md#stacks-ordering) docs |
| watch            | list(string)   | The list of `watch` files. See [change detection](../change-detection/index.md) for details |

## assert block schema

The `assert` block has no labels, **does not** support [merging](#config-merging),
can be defined multiple times and has the following schema:

| name             |      type      | description |
|------------------|----------------|-------------|
| assertion        | boolean        | If true assertion passed, fails otherwise |
| warning          | boolean        | True if the assertion is a warning |
| message          | string         | Message to show if assertion fails |

## globals block schema

The `globals` block accepts any number of labels, supports [merging](#config-merging), accepts **any** attribute and supports any number of
[map](#map-block) blocks.

For more information about `globals`, see the [Globals](../code-generation/variables/globals.md) documentation.

## map block schema

The `map` block can only be used inside the [globals](#globals-block-schema)
block, requires 1 label and optionally accepts a [value](#value-block-schema).

| name             |      type      | description |
|------------------|----------------|-------------|
| `for_each`        | `list(any)`       | The input list |
| `key`             | `string`          | The computed key |
| `value`           | `any`             | The value for the key |
| [`value`](#value-block-schema) | `block*` | value properties |

The `value` block and the `value` attribute **cannot** be used together.

## value block schema

The `value` block does not support labels. It accepts multiple attributes of any name and value of type `any`. It supports any number of nested [map](#map-block) blocks.

## generate_file block schema

The `generate_file` block requires one label, **do not** support [merging](#config-merging) and has the following schema:

| name             |      type      | description |
|------------------|----------------|-------------|
| [lets](#lets-block-schema) | block* | lets variables |
| condition        | bool           | The condition for generation |
| content          | string         | The content to be generated |


For detailed documentation about this block, see the [File Code Generation](../code-generation/generate-file.md) docs.

## generate_hcl block schema

The `generate_hcl` block requires one label, **do not** support [merging](#config-merging) and has the following schema:

| name             |      type      | description |
|------------------|----------------|-------------|
| [lets](#lets-block-schema) | block* | lets variables |
| condition        | bool           | The condition for generation |
| [content](#generate_hclcontent-block-schema) | block | The content to be generated |

For detailed documentation about this block, see the [HCL Code Generation](../code-generation/generate-hcl.md) docs.

## lets block schema

The `lets` block has no labels, supports [merging](#config-merging) of blocks
in the same level, accepts **any** attribute and supports any number of
[map](#map-block) blocks.

## generate_hcl.content block schema

The `generate_hcl.content` block has no labels and accepts any valid HCL.

## import block schema

The `import` block has no labels, **do not** supports [merging](#config-merging)
and has the following schema:


| name             |      type      | description |
|------------------|----------------|-------------|
| source           | string         | The file path to be imported |


## vendor block schema

The `vendor` block has no labels, **do not** support [merging](#config-merging)
and has the following schema:

| name             |      type      | description |
|------------------|----------------|-------------|
| dir              | string         | Absolute path relative to root where vendored projects will be downloaded |
| [manifest](#vendormanifest--block-schema) | block | The manifest for which files to vendor |

## vendor.manifest block schema

The `vendor.manifest` block has no labels, **do not** support [merging](#config-merging)
and has the following schema:

| name             |      type      | description |
|------------------|----------------|-------------|
| [default](#vendormanifestdefault--block-schema) | block | The default manifest |

## vendor.manifest.default block schema

The `vendor.manifest.default` block has no labels, **do not** support [merging](#config-merging)
and has the following schema:

| name             |      type      | description |
|------------------|----------------|-------------|
| files            | list(string)   | The list of patterns to match selected files. The pattern format is the same of [gitignore](https://git-scm.com/docs/gitignore#_pattern_format) |
