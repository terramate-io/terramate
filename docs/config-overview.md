# Terramate Configuration Overview

Different configurations can be done in Terramate,
ranging from avoiding duplication by leveraging powerful
code generation to flexible orchestration by allowing control
of stacks order of execution.

To do so, Terramate works with configuration files that
have the suffixes:

* `tm`
* `tm.hcl`

Terramate files can be found in any non-hidden directory of a Terramate project 
and all non-hidden files in a single directory will be handled as the 
concatenation of all of them in a single file, forming a single **configuration**.

A Terramate project is essentially a collection of Terraform code
organized into stacks.

It is not a hard requirement for Terramate to work that the project uses Git 
for version control (support to other VCS might be added in the future),
but features like change detection do depend on a VCS to
work and will fail if this soft requirement is not met.

In general, a Terramate project looks like this:

* A Git project.
* The git top-level dir is the project root dir.
* Stacks are organized as different directories.
* Configuration may be present on any directory.

# Terramate Configuration Schema

The terramate configuration defines the top-level blocks below:

- [terramate](#terramate-block-schema)
- [stack](#stack-block-schema)
- [globals](#globals-schema)
- [generate_file](#generate_file-schema)
- [generate_hcl](#generate_hcl-schema)

# terramate block schema

The `terramate` block has no labels and have the structure below:

| name             |      type      | description |
|------------------|----------------|-------------|
| required_version |     string     | [version constraint](https://www.terraform.io/language/expressions/version-constraints) |
| [config](#terramateconfig-block-schema) |     block      | project configuration |

## terramate.config block schema

The `terramate.config` block has no labels and have the structure below:

| name             |      type      | description |
|------------------|----------------|-------------|
| [git](#terramateconfiggit-block-schema) | block | git configuration |

## terramate.config.git block schema

The `terramate.config.git` block has no labels and have the structure below:

| name             |      type      | description |
|------------------|----------------|-------------|
| default\_branch | string | The default git branch |
| default\_remote | string | The default git remote |

# stack block schema

| name             |      type      | description |
|------------------|----------------|-------------|
| name             | string         | The name of the stack |
| description      | string         | The description of the stack |
| before           | **list(string)**   | The list of `before` stacks. See [ordering](https://github.com/mineiros-io/terramate/blob/main/docs/orchestration.md#stacks-ordering) docs. |
| after            | list(string)   | The list of `after` stacks. See [ordering](https://github.com/mineiros-io/terramate/blob/main/docs/orchestration.md#stacks-ordering) docs |
| wants            | list(string)   | The list of `wanted` stacks. See [ordering](https://github.com/mineiros-io/terramate/blob/main/docs/orchestration.md#stacks-ordering) docs |
