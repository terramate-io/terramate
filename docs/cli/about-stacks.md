---
title: An introduction to Terramate Stacks
description: Learn how stacks help you to split up your infrastructure code and configuration such as Terraform into isolated units.

prev:
  text: 'Quickstart'
  link: '/cli/getting-started/'

next:
  text: 'Stacks Configuration'
  link: '/cli/stacks/'
---

# About Stacks

A modular approach is recommended when working with Infrastructure as Code (IaC). This approach breaks the entire
infrastructure code and state into **smaller**, **isolated** units, often referred to as *stacks.*

A stack is a collection of infrastructure resources that you *configure, provision* and *manage* as a unit.

You can think about a stack as a combination of:

- **Infrastructure code** which declares a set of infrastructure assets and their properties.
  Terraform code (`.tf` files) and Cloud Formation (`.json` files) templates are both examples of infrastructure code.
- **State** that describes the status of the assets according to the *latest deployment* (e.g., Terraform state, Pulumi
  state, etc. - can be either stored locally or in a remote location)
- **Configuration** to *configure* the infrastructure assets and stack behaviour (e.g., Variables, Stack Configuration, etc.)

Using stacks to break up your infrastructure code into manageable pieces is considered an industry standard and provides the following benefits:

✅ **Reduce run times significantly** by selectively targeting only the required stacks for execution (e.g. only the stacks that have changed in the last PR). Stacks also enable the possibility of parallel execution.

✅ **Limit the blast radius risk** by grouping IaC-managed assets in logical units such as environments, business units,
regions or services that are isolated from each other.

✅ **Separate management responsibilities across team boundaries** by assigning and managing the ownership of stacks to
users and teams.

✅ **Remove sequential and blocking operations** by enabling parallel development and execution of independent stacks.

This page provides an overview of what Terramate Stacks are and how they help you create and manage composable Infrastructure as Code projects at any scale.

## What are Terramate Stacks?

Terramate Stacks are Infrastructure as Code agnostic and focus on adding functionality and features to improve the
**developer experience**, **productivity** and **scalability** in Infrastructure as Code projects at any scale.

You can use Terramate Stacks to manage IaC technologies such as Terraform, OpenTofu, Terragrunt, Kubernetes, AWS Cloud
Formation, AWS Cloud Development Kit  (CDK), Bicep, and others.

> **Note:** Some IaC technologies, such as AWS Cloud Development Kit (CDK) offer a native implementation of stacks,
> while others don’t. It’s important to understand that Terramate integrates seamlessly with those approaches.
> E.g., Terramate can be used to manage Terraform workspaces and CDK Stacks.

Most of the time, Terramate projects manage *dozens*, *hundreds*, or even *thousands* of stacks. This is possible because Terramate CLI provides a neat range of features that allow you to create and manage stacks efficiently at any scale:

- Stacks can be **created**, **cloned**, and **compared** with a single command.
- Stacks can be **orchestrated and targeted** for operations, which allows the execution of any command over a filtered section of stacks.
- The **change detection** allows the execution of only the stacks that contain changes.
- The **order of execution** of stacks can be configured explicitly in addition to the default order of execution.
- You can **generate code** in stacks. E.g. you can generate the Terraform backend configuration for all Terraform stacks or a Kubernetes manifest to create a secret for all Kubernetes stacks that follow certain criteria.
- Stacks can be used to **manage ownership** by leveraging concepts such as [CODEOWNERS](https://docs.github.com/en/repositories/managing-your-repositorys-settings-and-features/customizing-your-repository/about-code-owners).
- Stacks allow you to implement **multi-IaC** and **multi-step** scenarios.
- Since stacks always manage native infrastructure code, they **integrate all third-party tooling** seamlessly.

Stacks can be created with Terramate CLI using the create command and are directories containing a configuration file
configuring the metadata (`name`, `description`, `id`, `tags`, etc.) and [orchestration behavior](./orchestration/index.md) of the stack.

```hcl
# example-stack
stack {
  name        = "VPC"
  description = "Main VPC managed in europe-west2"
  id          = "780c4a63-79c2-4725-81f0-06d7c0435426"

  tags = [
    "terraform",
    "prd",
    "servive-abc"
  ]

  # Ensures that the current stack is executed before the following stacks
  before = [
    "../stack-a",
    "../stack-b",
  ]

  # Ensures that the current stack is executed after the following stacks
  after = [
    "../stack-c",
  ]

  # Forces the execution of a list of stacks whenever the current stack is executed
  # even if those don't contain any changes
  wants = [
    "../stack-d",
  ]
}
```

For an overview of all stacks configuration options available, please see the docs at [stacks configuration](./stacks/index.md).

## Summary

Stacks are a useful abstraction in IaC that allow us to define small units of assets. A stack consists of code, state and
configuration. The Terramate concept of stacks includes inheritance of configuration over the filesystem hierarchy and the
ability to run commands against a targeted set of stacks.
