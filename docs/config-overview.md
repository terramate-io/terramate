# Terramate Configuration

There is a series of different configurations that can be done
on Terramate, ranging from avoiding duplication by leveraging
powerful code generation to flexible orchestration by allowing
control of stacks order of execution.

In order to do so, Terramate works with configuration files that
have the suffixes:

* `tm`
* `tm.hcl`

Terramate files can be found in any directory of a Terramate project and
all the files in a single directory will be handled as the concatenation
of all of them in a single file.

Here we define the different kinds of configurations and
how you can learn more about them. But before getting into the different
kinds of configurations and all that you can do with Terramate,
what would be a Terramate project?

A Terramate project is essentially any collection of Terraform code
organized into stacks. You can have all Terraform code together in a single
directory but that would defeat the purpose of most Terramate features, so
usually, you have a set of stacks and maybe a set of Terraform modules if they
are not maintained in a different repository.

It is not a hard requirement for Terramate to work that the project uses git 
for version control, but features like change detection do depend on git to
work and will fail if this requirement is not met.

In general, a Terramate project looks like this:

* A git project
* The git top-level dir is the project root dir
* Stacks will be organized as different directories
* Configuration may be present on any dir

# Project Configuration

Per project configuration can be defined only once at the project root dir.

Available project-wide configurations can be found [here](project-config.md).

# Stack Configuration

Before talking about stack specific configuration, lets define what is a
Terramate stack:

* Has one or more Terramate configuration files.
* One of the configuration files has a `stack {}` block on it.
* It has no stacks on any of its subdirs (stacks can't have stacks inside them).

Here is the list of configurations specific to stacks:

* [Execution Ordering](execution-order.md)
* [Globals](globals.md)
* [HCL Generation](hcl-generation.md)
* [Code Generation](code-generation-config.md)
* [Locals Generation](locals-generation.md)
* [Backend](backend-config.md)
