---
title: Overview
description: Terramate supercharges Terraform with Stacks, Orchestration, Git Integration, Code Generation, Data Sharing and more.

next:
  text: 'Installation'
  link: '/installation'
---

# Introduction

Terramate supercharges Terraform with Stacks, Orchestration, Git Integration, Code Generation, Data Sharing and more.

It focuses on improving the Developer Experience (DX), providing workflows, and lowering the time spent writing and maintaining infrastructure code for projects at any scale.

Compared to other prominent tooling in the market, **Terramate always generates native Terraform code**
that integrates with existing tooling such as Terraform Cloud, Env0, Spacelift, Terragrunt, and others.

The main benefits of Terramate are:

1. **Stacks**: are isolated and independent units that contain infrastructure code such as Terraform,
Kubernetes Manifests and others. Stacks help you to split your monolithic IaC projects into smaller units
to reduce the blast radius, speed up execution time, provide clear ownership and better collaboration.

2. **Orchestration**: helps define the order of execution of commands such as `terraform plan|apply` in a selected
range of stacks. You can use orchestration to define the order of execution based on criteria such as tags or stacks
that contain changes.

3. **Git Integration**: allows you to execute commands (such as `terraform apply`) only against the stacks that have changed.
workflow and CI/CD pipelines and comes with advanced functionality, such as module change detection for Terraform stacks.

4. **Code Generation**: keep your Terraform DRY and maintainable by programmatically generating Terraform backend and provider
configurations. You can also provide blueprints for your teams to create stacks with pre-configured infrastructure.

5. **Data Sharing**: define data once and inherit, extend or overwrite it in your hierarchy of stacks. Extremely useful for
setting values such as tags or environments used in multiple stacks.

## Get started

To install Terramate please see the [Installation](https://terramate.io/docs/cli/installation) page in the documentation.

We recommend following our [Getting Started](https://terramate.io/docs/cli/getting-started/) guide to familiarize yourself
with the fundamentals by building your first Terramate project.

For a list of guides and examples available, please see the [guides](./guides/index.md) documentation page.

## Community

Join us on [GitHub](https://github.com/terramate-io/terramate) or [Discord](https://terramate.io/discord) to ask
questions, share feedback, meet other developers building with Terramate, and dream about the future of IaC.
