---
title: An introduction to Terramate CLI
description: Terramate CLI is an open-source Infrastructure as Code (IaC) orchestration tool for Terraform, OpenTofu, Terragrunt, Kubernetes, Pulumi, AWS Cloud Formation, AWS Cloud Development Kit (CDK), Azure Resource Manager (ARM), Biceps, and others.

next:
  text: 'Installation'
  link: '/cli/installation'
---

# Introduction

This page provides a high-level overview of what Terramate CLI is and how it works.

If you want to get started with a *practical introduction* and learn about the Terramate CLI API, head over to the
[Getting Started](./getting-started/index.md) guide.

<!-- To learn more about the motivation for Terramate, check out the Why Terramate? page. -->

## What is Terramate CLI?

Terramate CLI is an [open-source](https://github.com/terramate-io/terramate) Infrastructure as Code (IaC) orchestration tool for Terraform, OpenTofu, Terragrunt, Kubernetes, Pulumi, AWS Cloud Formation, AWS Cloud Development Kit (CDK), Azure Resource Manager (ARM), Biceps, and others.

Terramate CLI helps you to **unify**, **simplify** and **scale** all your infrastructure code, tools, and workflows and consists of the following parts:

- **Stacks:** Are IaC vendor agnostic and isolated units that group a bunch of infrastructure code, state, and configuration.
- **Orchestration**: Let’s orchestrate the execution of commands such as `terraform apply` in stacks.
- **Git Integration and Change Detection:** Helps you to detect and manage stacks that contain changes in a branch, commit or pull request.
- **Configuration:** Define and reuse data in stacks with environment variables, globals and metadata.
- **Code Generation:** Generate code in stacks to keep your stacks DRY and to provide pre-configured templates (think generating files such as Terraform provider configuration or Kubernetes manifests).

Terramate CLI can manage and orchestrate *any* Infrastructure as Code tool. It unlocks and simplifies **multi-step** and **multi-IaC** use cases and helps you implement and maintain scalable platforms.

## How does Terramate CLI work?

Terramate CLI is a dev tool that simplifies and helps manage Infrastructure as Code
(IaC) at any scale. We use the Hashicorp Configuration Language (HCL) as the
configuration language of choice to configure Terramate projects and stacks.

E.g., the following example demonstrates the configuration of a simple stack used to deploy and manage a simple Virtual Private Cloud (VPC) on AWS using Terraform.

```hcl
# /stacks/aws/vpc/europe-west1/main/stack.tm.hcl

#
# This is the configuration of an example Terramate stack
# used to deploy and manage a VPC with Terraform on AWS
#

stack {
  name        = "main-vpc"
  description = "Main VPC deployed in the us-west-2 region on AWS."
  id          = "07942413-6723-4a7d-9905-5e9de7c0288d"

  tags = [
    "terraform",
    "networking",
  ]
}
```

```hcl
# /stacks/aws/vpc/europe-west1/main/main.tf

#
# Terraform resource to deploy a simple VPC on AWS
#

resource "aws_vpc" "main" {
  cidr_block = "10.0.0.0/16"
}
```

The following sections explain the different features and components of Terramate CLI and how those help you efficiently
manage Infrastructure as Code projects at any scale.

## **Stacks**

Every Terramate project contains at least one stack. You can think about a stack as a combination of:

- **Infrastructure code** (e.g., Terraform, Pulumi, Cloud Formation, etc.)
- **State** of the managed infrastructure (e.g., Terraform state, Pulumi state, etc.)
- **Configuration** (environment variables, globals, etc.)

Most of the time, Terramate projects manage dozens, hundreds, or even thousands of stacks because splitting your IaC into smaller, isolated and manageable units unlocks the following benefits:

✅ **Significantly faster run times and lower costs**

✅ **Limit the blast radius and risk of infrastructure changes**

✅ **Better ownership management and governance**

✅ **Improved productivity and developer experience while reducing complexity**

✅ **Better observability and security**

✅ **Enables multi-step and multi-IaC use cases**

Stacks in Terramate can be cloned, nested, compared and orchestrated. You can also generate code in stacks to keep them DRY (e.g., generate files such as the Terraform backend configuration for all stacks that manage Terraform resources).

<img src="./assets/stacks.png" width="600px" alt="Terramate Stacks Overview" />


## **Orchestration**

Stacks enable many benefits and simplify provisioning and managing resources at scale, reducing the time and overhead of managing infrastructure. But how can we execute commands such as `terraform` or `kubectl` in stacks?

That’s where the Terramate CLI orchestration engine comes into place. It’s a powerful feature that helps us orchestrate and execute commands in all stacks that fulfill certain criteria.

E.g., to invoke commands in all stacks, all it needs is a single command.

```sh
terramate run <CMD>
```

> One of the biggest differences between Terramate CLI and other, rather vendor centric tooling such as e.g. Terragrunt is that Terramate CLI doesn’t wrap any specific IaC tool. Instead, Terramate CLI can be used to execute and orchestrate any command and binary.
E.g. you can use `terramate run -- pwd` to run `pwd` in all stacks.
> 

The [run](cmdline/run.md) command accepts filters such as tag or directories in order to limit the range of executed stacks, e.g.

```sh
# Runs kubectl in all stacks that are tagged with kubernetes AND prd
terramate run --tags kubernetes:prd -- kubectl diff

# Runs terraform in all stacks that are tagged with terraform OR opentofu
terramate run --tags terraform,opentofu -- terraform init

# Runs terraform init in all stacks in stacks/aws/vpc that are tagged with terraform
terramate run -C stacks/aws/vpc --tags terraform -- terraform init
```

Whenever orchestrating multiple stacks, Terramate CLI allows you to define the order of execution explicitly, in addition to its default behavior, with the [`before`, `after`and `wants`](orchestration/index.md) statements.

```hcl
# /stacks/aws/vpc/europe-west1/main/stack.tm.hcl
#
# This is the configuration of an example Terramate stack
# used to deploy and manage a VPC with Terraform on AWS
#

stack {
  name        = "main-vpc"
  description = "Main VPC deployed in the us-west-2 region on AWS."
  id          = "07942413-6723-4a7d-9905-5e9de7c0288d"

  tags = [
    "terraform",
    "networking",
  ]

  # Ensures that the current stack is executed before the following stacks
  before = [
    "../stack-a",
  ]

  # Ensures that the current stack is executed after the following stacks
  after = [
    "../stack-b",
  ]

  # Forces the execution of a list of stacks whenever the current stack is executed
  # even if those don't contain any changes
  wants = [
    "../stack-c",
  ]

  # Ensures that the current stack always gets executed when a list of configured
  # stacks are executed even if the current stack doesn't contain any changes
  # wants_by = [
  #   "../stack-d",
  # ]
}
```

If you want to understand and debug the order of execution of stacks in your Terramate project, the [run-order](cmdline/run-order.md) command will come in handy.

## Git Integration and Change Detection

One of the most powerful features of Terramate CLI is its ability to **filter for stacks that contain changes** in the recent commit, current branch, or Pull Request.

Combining stacks and change detection enables us to significantly improve the performance of our Infrastructure as Code projects by, e.g., executing commands in stacks that contain changes only instead of always running the entire environment, as well as adding the ability to execute stacks that don’t depend on each other in parallel.

The change detection in Terramate CLI is based on git and works by computing the changed stacks from the changed files discovered by the `git diff` between the revision of the last change (i.e. the released revision) and the current change.

You can use the `--changed` flag to filter for stacks that contain changes only.

```sh
terramate run --changed -- <CMD>
```

This can also be combined with other filters such as the `--tags` filter.

```sh
terramate run --changed --tags terraform -- terraform apply
```

To learn more about how the change detection works please see the [documentation](change-detection/index.md).

## Configuration

When working with multiple stacks (and multiple IaC tools), using a standardized way to configure shared data among stacks is required. Terramate CLI has various options that allow you to define data once and distribute it throughout your project using hierarchical merge semantics.

- Environment Variables
- Globals
- Metadata

```hcl
# config.tm.hcl

globals {
  providers = {
    aws    = "~> 5.29"
    google = "~> 5.8"
  }
}
```

> [Environment Variables](configuration/project-config.md#the-terramate-config-run-env-block) and **[Globals](data-sharing/globals.md)** are user-defined entities, whereas **[Metadata](data-sharing/metadata.md)** is information supplied by Terramate CLI itself.
> 

Terramate CLI defined configuration can also be used inside the code generation, which is pretty useful when you, e.g., automatically generate files such as Terraform provider and backend configuration for stacks.

## Code Generation

Terramate CLI allows us to generate all kind of code and files using its built in code generation feature.

```hcl
generate_file "hello_world.json" {
  content = tm_jsonencode({"hello" = "world"})
}

generate_file "hello_world.yml" {
  content = tm_yamlencode({"hello" = "world"})
}
```

You can use *user* and *Terramate defined* configuration such as [Globals](data-sharing/globals.md) and [Metadata](data-sharing/metadata.md) inside the code generation which comes really handy, when you e.g. want to automatically generate files for stacks that fullfil certain criteria.

One use case would be to automatically generate Terraform backend and provider configuration for all stacks that manage Terraform. 

```hcl
# config.tm.hcl

globals "terraform" "backend" "s3" {
  region = "us-west-2"
  bucket = "terramate-terraform-example-state"
}
```

```hcl
# terraform_backend.tm.hcl

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

> To see a complete guide on how to leverage the code generation to keep Terraform stacks DRY, please [see](https://github.com/terramate-io/terramate-examples/tree/main/01-keep-terraform-dry).
> 

The code generation in Terramate CLI is a very powerful and flexible feature that has much more to offer than what we just demonstrated in the above mentioned example. To dive in further please see the [Code Generation](code-generation/index.md) documentation.

## Terramate Cloud

[Terramate Cloud](https://terramate.io/) provides the developer experience and infrastructure to build, scale and observe all your infrastructure managed with IaC.

Terramate Cloud is a platform that provides a **dashboard**, **observability** and **insights**, **deployment metrics** (e.g., **DORA**), **notifications**, **drift management**, **asset management,** and more to help manage Infrastructure as Code with stacks at scale.

It also provides automation and collaboration workflows that run natively in your existing CI/CD pipelines such as GitHub Actions, Bitbucket Pipelines and GitLab in a secure and cost effective manner.

![Terramate Cloud Dashboard](./assets/terramate-cloud-dashboard.png "Terramate Cloud Dashboard")

If you are interested in Terramate Cloud, please [book a demo](https://terramate.io/demo/) or get in touch via email at hello@terramate.io or [discord](https://terramate.io/discord).

## Community

Join us on [GitHub](https://github.com/terramate-io/terramate) or [Discord](https://terramate.io/discord) to ask
questions, share feedback, meet other developers building with Terramate, and dream about the future of IaC.
