---
title: Stacks | Terramate
description: Terramate adds powerful capabilities such as code generation, stacks, orchestration, change detection, data sharing and more to Terraform.

prev:
  text: 'Getting Started'
  link: '/getting-started/'

next:
  text: 'Orchestration'
  link: '/orchestration/'
---

# Stack Configuration

When working with Infrastructure as Code (IaC), adopting a modular approach is highly recommended. This approach breaks the entire IaC into smaller, isolated stacks, enabling code and infrastructure component reuse. 

Additionally, it enhances infrastructure management across multiple stacks and facilitates testing and deploying changes in a controlled manner, minimizing unintended consequences.

The Terramate CLI stack is a powerful feature of the Terramate CLI tool, allowing you to manage complex deployments with ease.


## What is a Stack?

A stack is a collection of related resources managed as a single unit. When defining a stack, you specify all included resources, such as networks, virtual machines, and storage. 

Terramate CLI simplifies resource creation and management, allowing you to focus on building and deploying the application.


## A Terramate stack is:

- A directory inside your project
- Contains one or more TerraMate configuration files
- Includes a configuration file with a stack{} block

The `stack{}` block distinguishes a stack from other directories in Terramate. By default, it doesn't require any attributes but can be used to describe stacks and orchestrate their execution.

Stack configurations related to orchestration can be found [here](../orchestration/index.md).

The `stack{}` block also defines attributes used to describe the stack. Only [Terramate Functions](../functions/index.md) are available when defining
the `stack` block.

## Why use Stacks?

Stacks offer several benefits:

1. Manage multiple resources as one unit, allowing you to build and deploy the entire infrastructure efficiently and quickly with a single command.

2. Easily manage dependencies between resources. For example, you can specify a virtual machine that depends on a virtual network, and TerraMate CLI will automatically manage the resource's creation or deletion.

3. Simplify infrastructure management as code. Defining the entire infrastructure using Terraform configuration files makes version control and change management more accessible.


## Creating a Stack:

To create a stack using Terramate, follow these steps:

1. Define the included resources in a Terraform configuration file. This file should contain all resources you want to manage as part of the stack.

2. Use the following command to create the resources:

```hcl
terramate stack create
```
Running this command creates all defined resources.

Managing Stacks

After creating a stack, you can use Terramate CLI to manage it. To update the stack, use:
terramate stack update

- To delete the entire stack you can use the command
```hcl
terramate stack delete
```

- You can also view the status of your stacks by using the following command
```hcl
terramate stack status
```
Running this command displays the status of all resources, allowing you to quickly configure resources that require urgent attention.


# Properties
Each stack has a set of properties that can be accessed and used while building your IaC. Some of these properties include:

## stack.id (string)(optional)

The stack ID **must** be a string composed of alphanumeric chars + `-` + `_`.
The ID can't be bigger than 64 bytes and **must** be unique on the
whole project.

There is no default value determined for the stack ID.

Eg:

```hcl
stack {
  id = "some_id_that_must_be_unique"
}
```

## stack.name (string)(optional)

The stack name can be any string and defaults to the stack directory base name.

Eg:

```hcl
stack {
  name = "My Awesome Stack Name"
}
```

## stack.description (string)(optional)

The stack description can be any string and defaults to an empty string.

Eg:

```hcl
stack {
  description = "My Awesome Stack Description"
}
```

## stack.tags (set(string))(optional)

The tags must be a unique set of strings, where each tag must adhere to the following rules:

- It must start with a lowercase ASCII alphabetic character (`[a-z]`).
- It must end with a lowercase ASCII alphanumeric character (`[0-9a-z]`).
- It must have only lowercase ASCII alphanumeric, `_` and `-` characters (`[0-9a-z_-]`).

## stack.watch (list)(optional)

The list of files that must be watched for changes in the
[change detection](../change-detection/index.md).

## stack.after (set(string))(optional)

The `after` defines the list of stacks which this stack must run after.
It accepts project absolute paths (like `/other/stack`), paths relative to
the directory of this stack (eg.: `../other/stack`) or a [Tag Filter](../tag-filter.md).
See [orchestration docs](../orchestration/index.md#stacks-ordering) for details.

## stack.before (set(string))(optional)

Defines the list of stacks that this stack must run `before`.
It accepts project absolute paths (like `/other/stack`), paths relative to
the directory of this stack (eg.: `../other/stack`) or a [Tag Filter](../tag-filter.md).
See [orchestration docs](../orchestration/index.md#stacks-ordering) for details.
