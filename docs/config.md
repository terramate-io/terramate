# Terramate Project Configuration

There is a series of different configurations that can be done
on terramate, ranging from avoiding duplication to controlling
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

## Project Configuration

## Stack Configuration

## Hierarchical Configuration

# Configurations

Each kind of configuration will have its own particular semantics, like where
it can be defined and how it composes with other configurations:

* [Project](project-config.md)
* [Globals](globals.md)
* [Metadata](metadata.md)
* [Execution Ordering](execution-order.md)
