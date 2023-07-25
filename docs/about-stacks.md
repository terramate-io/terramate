---
title: An introduction to Stacks
description: Learn about stacks in Terramate and why it's advised to use stacks in your infrastructure as code projects.

prev:
  text: 'Quickstart'
  link: '/getting-started/'

next:
  text: 'Stacks Configuration'
  link: '/stacks/'
---

# About Stacks

The stack concept is advised for several reasons. But before we go into
**why** lets talk a little about **what** a stack is.

The stack concept is not defined by Hashicorp's Terraform tooling but just a
convention used by the _Terraform community_, so a stack can be loosely defined as:

```
A terraform stack is a runnable terraform module that operates on a subset of
the infrastructure's resources.
```

By _runnable_ it means it has everything needed to call
`terraform init/plan/apply` . For example, a module used by other modules, ie,
don't have the `provider` explicitly set, is not runnable hence it's
**not a stack**.

If a module is missing any required components, such as provider settings,
it cannot be considered a stack.

It's also worth noting that if a module creates your entire infrastructure, it's not a stack.
**Stacks are designed to break down your infrastructure into** **manageable pieces**,
so they operate on a specific subset of resources rather than your entire infrastructure.


## Isolate code that changes frequently

If you make a lot of updates to your infrastructure, like deploying new code or changing
settings often, doing everything in one Terraform project could cause problems.
This might lead to some parts of your infrastructure needing to be rebuilt, which means
more downtime.

Using stacks helps with this issue. Stacks let you only update the parts of
your system that need changes, without affecting everything else. Be careful to
make the stacks the right size to avoid unnecessary repetition.

In short, try to keep code that changes for different reasons separate, so you
don't create problems when making updates.

## Reduce the blast radius

A small change can lead to catastrophic events if you're not careful. For instance,
forgetting a `prevent_destroy` attribute in the production database lifecycle
declaration. Also, someone within your organisation or team may commit mistakes
and it's better if you could reduce the impact of such errors.
An extreme example is: avoiding a database instance destroy because of a DNS TTL
change.

## Reduce execution time

Using stacks allows you to reduce the runtime of Terraform drastically. This is especially
beneficial when running Terraform in CI/CD environments like GitHub Actions since build
time is usually billed per minute, which means faster execution time translates directly
to lowered costs.

## Ownership

In the case of a big organization you probably don't want a single person or
team responsible for the whole infrastructure. The company's stacks can be
spread over several repositories and managed by different teams.

By having this separation, it also makes it easier when you want separation
by resource permissions, so having some stacks that can only be run by
specific individuals or teams.

## Others

There are lots of other opinionated reasons why stacks would be a good option:
stacks per environment or deploy region location, stacks per topic (IAM vs
actual resources) and so on.
