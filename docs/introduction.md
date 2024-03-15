---
title: What is Terramate?
description: Terramate is an Infrastructure as Code Management Platform that empowers teams to automate, orchestrate and observe IaC such as Terraform, OpenTofu, Terragrunt and Kubernetes.
---

# About Terramate

Terramate is an Infrastructure as Code (IaC) Management Platform that empowers teams to **organize**, **automate**, **orchestrate** and **observe** IaC tools such as Terraform, OpenTofu, Terragrunt, Kubernetes and others.

It's designed and implemented by long-time Platform Engineering and DevOps practitioners based on previous experience
building and maintaining cloud infrastructure for some of the most innovative companies in the world.

Terramate takes you from zero to fully managing and automating your cloud resources with infrastructure as code in less
than 5 minutes, with no prerequisites, no lock-in and without modifying any of your existing configurations.

At the same time, it integrates nicely and in a non-intrusive way with all your existing tooling, such as GitHub or Slack.

## What is Terramate

Using Terramate provides you with a scalable and cost-efficient approach to manage cloud infrastructure with
Infrastructure as Code tools you already know and use by imposing **structure**, **workflows** and **best practices**.

Terramate combines a free and [open source](https://github.com/terramate-io/terramate) CLI used by developers or in
automation and optionally pairs with a fully managed Cloud service [Terramate Cloud](./cloud/index.md).

![Terramate Cloud Dashboard](./cloud/assets/dashboard.png "Terramate Cloud Dashboard")

::: tip
Terramate is **not** a CI/CD platform. It integrates with your existing CI/CD such as GitHub Actions, GitLab CI/CD,
Bitbucket, Azure DevOps, TeamCity, Circle CI and Jenkins, allowing you to reuse compute, access management, etc. to
automate and orchestrate your IaC using GitOps workflow in a secure and cost-effective manner!
:::

## Benefits

Terramate allows you to automate, audit, secure, and continuously deliver your infrastructure by using a platform that is
successfully used by some of the best Platform Engineering and DevOps teams in the world .

1. **Built-in automation**: Enables you to adopt GitOps for Infrastructure in any CI/CD
4. **Faster and more secure deployments**: Limit the blast radius and risk of infrastructure changes
3. **Improved productivity and developer experience**: Removes guess-work and config sprawl by imposing structure, workflows and best practices
2. **Secure, scalable and cost-efficient**: Build on top of a battle-tested and proven platform instead of wasting time
building and maintaining a DIY solution. By integrating with your existing CI/CD instead of replicating it
5. **Zero maintenance overhead**: Serverless, with Terramate you don't need to host or maintain any infrastructure
6. **No lock-in and seamless integrations**: Using Terramate doesn't require to touch any of your existing configurations and integrates seamlessly with all other tooling

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
- **Git Integration and Change Detection:** Detect and manage stacks that contain changes in a branch, commit or pull request.
- **Configuration:** Define and reuse data in stacks by using variables and metadata.
- **Code Generation:** Generate code in stacks to keep your stacks DRY and to provide pre-configured templates
(think of generating files such as Terraform provider configuration or Kubernetes manifests).
- **Native Infrastructure as Code**: Terramate doesn't introduce any complex wrappers or abstraction layers and allows
you to stay in native environments (e.g. native Terraform).
- **Unlimited Integrations**: Terramate integrates with all existing tooling. OPA, Infracost, Checkov, Trivy, Terrascan, etc - you name it.


### Terramate Cloud

Terramate Cloud is a fully managed service that provides **collaboration**, **observability**, **visibility** and **governance**.
It is free for individual use, with features available for teams.

- **Dashboard**: Understand and observe the state of all your IaC among multiple repositories and cloud accounts.
- **Asset Management**: Understand and control all infrastructure as code assets and resources
- **Collaboration**: Teams regain ownership and can collaborate on every step of the IaC deployment lifecycle. Discuss
planned changes through previews, and take ownership of failures and drifts.
- **Observability and Visibility**: Understand the status of your Infrastructure stacks. Into application state, resource usage, cost, ownership, changes, access, associated deployments and more. See inside any stack, deployment, and resource, at any scale, anywhere.
- **Drift Control**: Automatically detect and resolve drifts.
- **Governance**: Manage ownership for stacks, deployments, failure, drift and more.
- **Terramate AI**: LLM-powered analysis of deployment status, blast radius and more.

## Next steps

If you are new to Terramate, please take some time to read the following pages and sections. If you are more advanced,
you can navigate directly to the individual sections you need, or use the search feature to find specific information.

If you have questions, feel free to join our [Discord Community](https://terramate.io/discord) or
[book a demo](https://terramate.io/discord) with one of our experts.
