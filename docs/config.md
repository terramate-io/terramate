# Terramate Configuration

There is a series of different configurations that can be done
on Terramate, ranging from avoiding duplication to controlling
order of execution of stacks.

In order to do so, Terramate works with configuration files named
**terramate.tm.hcl**. These files can be found in any directory
of a Terramate project, but some configurations will be defined
only once for an entire project, while others provide merge/overriding
strategies, while others control only behavior specific for
stacks.

Here is defined the different kinds of configurations and
how you can learn more about them. But before getting into the different
kinds of configurations, what would be a Terramate project?

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
* The git top-level dir is the project base dir
* Stacks will be organized as different directories
* Configuration may be present on any dir

# Project Configuration

Per project configuration can be defined only once at the project base dir.

Available project-wide configurations can be found [here](project-config.md)

# Stack Configuration

Before talking about stack specific configuration, lets define what is a
Terramate stack:

* Has a valid terramate configuration file (**terramate.tm.hcl**).
* The terramate configuration file has a `stack {}` block on it.
* It has no stacks on any of its subdirs (stacks can't have stacks inside them).

Stacks have configurations that are particular to them, like these:

* [Execution Ordering](execution-order.md)

[Metadata](metadata.md) can be used on any hierarchical configuration,
it provides information that is useful to stack configuration and is
always evaluated on the context of a stack.

# Hierarchical Configuration

Hierarchical configuration is all configuration that can be defined on
any Terramate dir, with each kind of configuration having different semantics
on how overriding/merging happens when multiple configurations are
present across the project.

The following configurations have hierarchical behavior:

* [Globals](globals.md)
* [Backend](backend-config.md)
