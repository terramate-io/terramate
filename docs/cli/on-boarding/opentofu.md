---
title: "On-boarding: OpenTofu On-boarding"
description: Import your existing OpenTofu Setup to Terramate
---

# Start with existing OpenTofu Projects

## Import Existing OpenTofu Stacks

To create Terramate Stacks from existing OpenTofu Root Modules run the following command.

At the moment we support OpenTofu over the Terraform Integration as most details are still identical.

```bash
terramate create --all-terraform
```

This command will detect existing OpenTofu Root modules and create a stack configuration in them.

## Terramate Features for OpenTofu Repositories

All Terramate features are now available to your team.

The following set of features highlights some special benefits:

- Use Terramate Change Detection to orchestrate OpenTofu in an efficient way
- Execute **any** command within stacks imported from opentofu configuration.
- Run OpenTofu in any CI/CD following the Terramate Automation Blueprints and examples.
- Make use of Terramates advanced Code Generation and Globals to share data more easily.
- Synchronize deployments, drift runs, and previews to **Terramate Cloud** and get
  - Visibility of the Health of all OpenTofu Configurations over multiple repositories
  - Drift Detection in all Stacks
  - Pull Request Previews for actual changes
  - Notifications on deployment failures or newly detected drifts
  - Advanced collaboration and alert routing

## Run OpenTofu Commands

### List all Stacks

Any Terramate CLI Feature is now available in your Stacks.

```bash
terramate list
```

### Init OpenTofu

```bash
terramate run -- tofu init
```

### Create a OpenTofu Plan in parallel

```bash
terramate run --parallel 5 -- tofu plan -out plan.otplan
```

### Apply a OpenTofu Plan in Changed Stacks

```bash
terramate run --changed -- tofu apply -out plan.otplan -auto-approve
```
