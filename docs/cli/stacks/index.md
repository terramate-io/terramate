---
title: An Introduction to Stacks
description: Learn how stacks help you efficiently build and manage infrastructure as code projects at any scale with technologies such as Terraform.
---

# About stacks

A modular approach is recommended when working with Infrastructure as Code (IaC). This approach breaks the entire infrastructure code and state into **smaller** and **isolated** units, often referred to as **_stacks._**

## What are stacks?

A stack is a collection of infrastructure resources that you _configure, provision_ and _manage_ as a unit.

You can think about a stack as a combination of:

- **Infrastructure code** which declares a set of infrastructure assets and their configuration.
  Terraform code (`.tf` files) and Cloud Formation (`.json` files) templates are examples of infrastructure code.
- **State** that describes the status of the assets according to the _latest deployment_ (e.g., Terraform state,
  Pulumi state, etc. - can be either stored locally or in a remote location)
- **Configuration** to _configure_ the infrastructure assets and stack behavior (e.g., Variables, Stack Configuration, etc.)

Using stacks to break up your infrastructure code into manageable pieces is considered an industry standard and
provides the following benefits:

**✅ Reduce run times significantly** by selectively targeting only the required stacks for execution (e.g., only the
stacks that have changed in the last PR). Stacks also enable the possibility of parallel execution.

✅ **Limit the blast radius risk** by grouping IaC-managed assets in logical units such as environments, business units,
regions or services isolated from each other.

✅ **Separate management responsibilities across team boundaries** by assigning and managing the ownership of stacks to
users and teams.

✅ **Remove sequential and blocking operations** by enabling parallel development and execution of independent stacks.

## What are Terramate Stacks?

Terramate Stacks are Infrastructure as Code agnostic stacks and improve the **developer experience**, **productivity**
and **scalability** in Infrastructure as Code projects of any scale.

You can use Terramate Stacks to manage IaC technologies such as Terraform, OpenTofu, Terragrunt, Kubernetes, AWS Cloud
Formation, AWS Cloud Development Kit (CDK), Bicep, and others.

::: info
Some IaC technologies, such as AWS Cloud Development Kit (CDK), offer native implementations of stacks, while others don’t.
It’s important to understand that Terramate integrates seamlessly with those approaches.
E.g., Terramate can be used to manage Terraform workspaces and CDK Stacks.
:::

Most of the time, Terramate projects manage _dozens_, _hundreds_, or even _thousands_ of stacks. This is possible
because Terramate CLI provides a neat set of features that allow you to create and manage stacks efficiently at any
scale:

👉 Stacks can be **created**, **cloned**, and **compared** with a single command.

👉 Stacks can be **orchestrated and targeted** for operations, which allows the execution of any command
(e.g., `terraform apply`) over a filtered selection of stacks.

👉 The **change detection** allows the execution of only the stacks that contain changes.

👉 The **order of execution** of stacks can be configured explicitly in addition to the default order of execution.

👉 You can **generate code** in stacks. E.g. you can generate the Terraform backend configuration for all Terraform stacks
or a Kubernetes manifest to create a secret for all Kubernetes stacks that follow certain criteria.

👉 Stacks can be used to **manage ownership** by leveraging concepts such as
[CODEOWNERS](https://docs.github.com/en/repositories/managing-your-repositorys-settings-and-features/customizing-your-repository/about-code-owners).

👉 Stacks allow you to implement **multi-IaC** and **multi-step** scenarios.

👉 Since stacks always manage native infrastructure code, they **integrate all third-party tooling** seamlessly.

Stacks can be created with the [create](../cmdline/create.md) command, which creates a directory and a configuration file
`stack.tm.hcl` used to configure the metadata (`name`, `description`, `id`, `tags`, etc.),
[orchestration](../orchestration/index.md#order-of-execution) and [change detection behavior]() of the stack.

```hcl
# stack.tm.hcl
stack {
  # ~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~
  # Configure the metadata of a stack
  # ~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~
  name        = "Terraform Example Stack"
  description = "An awesome stack for demo purposes"
  id          = "780c4a63-79c2-4725-81f0-06d7c0435426"

  tags = [
    "terraform",
    "prd",
    "service-abc"
  ]

  # ~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~
  # Optionally the orchestration behavior can be configured
  # ~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~

  # Ensures that the current stack is executed before the following stacks
  before = [
    "../stack-a",
    "../stack-b",
  ]

  # Ensures that the current stack is executed after the following stacks
  after = [
    "../stack-c",
  ]

  # ~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~
  # Optionally the trigger behavior can be configured
  # ~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~

  # If any of the configured files changed, this stack will be marked as changed in the change detection.
  watch = [
    "/policies/mypolicy.json"
  ]

  # Forces the execution of a list of stacks whenever the current stack is executed
  # even if those don't contain any changes
  wants = [
    "../stack-d",
  ]

  # Ensures that the current stack always gets executed when a list of configured
  # stacks are executed even if the current stack doesn't contain any changes
  wanted_by = [
    "../stack-e",
  ]
}
```

For an overview of all stacks configuration options available, please see the docs in
[stacks configuration](configuration.md).

## Summary

Stacks are a useful abstraction in Infrastructure as Code that allows us to define small units of assets. A stack consists
of infrastructure code, state and configuration. The Terramate concept of stacks includes the inheritance of configuration
over the filesystem hierarchy and the ability to orchestrate commands in a targeted set of stacks.
