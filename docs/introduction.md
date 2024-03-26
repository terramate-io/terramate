---
title: What is Terramate?
description: Terramate is an Infrastructure as Code Management Platform that empowers teams to automate, orchestrate and observe IaC such as Terraform, OpenTofu, Terragrunt and Kubernetes.
---

# About Terramate

Terramate enables teams to **build**, **deploy**, **manage** and **observe** cloud infrastructure with Infrastructure as
Code (IaC) tools such as Terraform, OpenTofu, Terragrunt, Kubernetes and others.

Infrastructure as Code is hard and complex - we get it! 

Terramate helps you to overcome this by providing a ***next-generation*** Infrastructure as Code Management Platform that
runs natively in your existing CI/CD system.

Using Terramate makes it possible to:

1. **Streamline configuration management and environments**: Terramate enables you to manage environments and services with
stacks and adds code generation to keep configuration DRY, which means you can easily deploy multiple services and environments
and avoid manual copy & paste of duplicate configuration files.
2. **Automate and orchestrate Terraform, OpenTofu and Terragrunt in any CI/CD**: Terramate enables you to turn any CI/CD into a powerful
IaC vending machine which means no more wasted time and money maintaining DIY solutions or buying expensive purpose-built
CI/CD systems.
3. **Enable better collaboration, governance, observability and drift control**: Terramate adds a powerful suite of features
to improve the developer experience when working with infrastructure as code such as plan previews, policies, observability,
notifications, SlackOps, automated drift detection and reconciliation, which means better productivity, fewer failures and more stable infrastructure.

Terramate is designed and implemented by long-time Platform, Cloud and DevOps Engineering practitioners based on previous experience
building and maintaining cloud infrastructure platforms for some of the most innovative companies in the world.

It can be integrated into any existing architecture and with every third-party tooling in less than 5 minutes,
with no prerequisites, no lock-in and without modifying any of your existing configurations.

At the same time, Terramate integrates seamlessly and in a non-intrusive way with all your existing tooling, such as GitHub or Slack.

## What is Terramate

Terramate combines a free and [open source](https://github.com/terramate-io/terramate) CLI used by developers and in
automation that optionally pairs with a fully managed Cloud service [Terramate Cloud](./cloud/index.md).

![Terramate Cloud Dashboard](./cloud/assets/dashboard.png "Terramate Cloud Dashboard")

::: tip
Terramate is **not** a CI/CD platform. It integrates with your existing CI/CD such as GitHub Actions, GitLab CI/CD,
Bitbucket, Azure DevOps, Atlantis, Circle CI, Jenkins and others, allowing you to reuse existing pull request workflows, compute,
access management, and more to automate and orchestrate your IaC using GitOps workflow in a secure and cost-effective manner!
:::

## Benefits

Terramate allows you to automate, audit, secure, and continuously deliver your infrastructure by using a platform that is
successfully used by some of the best Platform Engineering and DevOps teams in the world.

1. **Built-in automation**: Enables you to adopt GitOps for Infrastructure in any CI/CD.
4. **Faster and more secure deployments**: Limit the blast radius and risk of infrastructure changes.
3. **Improved productivity and developer experience**: Removes guess-work and config sprawl by imposing structure, workflows and best practices.
2. **Secure, scalable and cost-efficient**: Build on top of a battle-tested and proven platform instead of wasting time.
building and maintaining a DIY solution. By integrating with your existing CI/CD instead of replicating it. Terramate is
secure by design and never needs access to your state or any of your cloud accounts.
5. **Zero maintenance overhead**: Serverless, with Terramate you don't need to host or maintain any infrastructure.
6. **No lock-in and seamless integrations**: Using Terramate doesn't require to touch any of your existing configurations
and integrates seamlessly with all other tooling.

## Features

Terramate helps manage cloud infrastructure with infrastructure as code better by providing a suite of powerful tools to
address common challenges in Infrastructure as Code.

### Terramate CLI

Terramate CLI is an orchestration and code generation tool that enables you to **unify**, **simplify**
and **scale** all your infrastructure code, tools and workflows.

- **Stacks:** Are Infrastructure as Code tooling agnostic and isolated units used to group, deploy and manage
infrastructure resources such as a single service or an entire environment.
- **Orchestration**: Allows orchestrating the execution of commands such as `terraform apply`, `kubectl apply` or
`infracost breakdown` in stacks. 
- **GitOps for Infrastructure**: Pre-configured and fully customizable GitOps automation workflows that run in your existing CI/CD platform.
- **Workflows**: Define custom commands to combine multiple commands into one executable unit of work.
- **Configuration:** Define and reuse data in stacks by using variables and metadata.
- **Code Generation:** Generate code in stacks to keep your stacks DRY and to provide pre-configured templates
(think of generating files such as Terraform provider configuration or Kubernetes manifests).
- **Native Infrastructure as Code**: Terramate doesn't introduce any complex wrappers or abstraction layers and allows
you to stay in native environments (e.g. native Terraform).
- **Unlimited Integrations**: Terramate integrates with all existing tooling. OPA, Infracost, Checkov, Trivy, Terrascan, Terragrunt, etc - you name it.


### Terramate Cloud

Terramate Cloud is a fully managed service that provides **collaboration**, **observability**, **visibility** and **governance**.
It is free for individual use, with features available for teams.

- **Dashboard**: Understand and observe the state of all your IaC among multiple repositories and cloud accounts.
- **Asset Management**: Understand and control all infrastructure as code assets and resources.
- **Collaboration**: Teams regain ownership and can collaborate on every step of the IaC deployment lifecycle. Discuss
planned changes through previews, and take ownership of failures and drifts.
- **Observability and Visibility**: Understand the status of your Infrastructure stacks. Into application state, resource usage, cost, ownership, changes, access, associated deployments and more. See inside any stack, deployment, and resource, at any scale, anywhere.
- **Terramate AI**: LLM-powered analysis of deployment status, blast radius, and more.
- **Drift Control**: Automatically detect and resolve drifts.
- **Governance**: Manage ownership for stacks, deployments, failure, drift and more.

## Next steps

If you are new to Terramate, please take some time to read the following pages and sections. If you are more advanced,
you can navigate directly to the individual sections you need, or use the search feature to find specific information.

If you have questions, feel free to join our [Discord Community](https://terramate.io/discord) or
[book a demo](https://terramate.io/discord) with one of our experts.
