# Terramate Project Configuration

There is a series of different configurations that can be done
on Terramate, ranging from avoiding duplication to controlling
order of execution of stacks.

In order to do so Terramate works with configuration files named
**terramate.tm.hcl**. These files can be found on any directory
of a Terramate project, but some configurations will be defined
only once for an entire project, while others provide merge
strategies, while others control only behavior specific for
stacks.

Given that it is important to define the different kinds of
configurations and their respective behavior. First lets define
some concepts:

# Concepts

## Project

A Terramate project is essentially any project containing terraform code
organized into stacks. You can have all terraform code together in a single
directory but that would defeat the purpose of most Terramate features, so
usually you have a set of stacks and maybe a set of terraform modules if they
are not maintained in a different repository.

It is not a hard requirement for Terramate to work that the project uses git 
for version control, but features like change detection do depend on git to
work and will fail if this requirement is not met.

So in general, a Terramate project looks like this:

* A git project
* The git top level dir is the project base dir
* Stacks will be organized as different directories
* Configuration may be present on any dir

## Project Configuration

## Stack Configuration

## Hierarchical Configuration

# Configurations

Each kind of configuration will have its own particular semantics, like where
it can be defined and how it composes with other configurations:

* [Project](project-config.md)
* [Globals](globals.md)
* [Backend](backend-config.md)
* [Metadata](metadata.md)
* [Execution Ordering](execution-order.md)
