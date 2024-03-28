---
title: "On-boarding: Terraform On-boarding"
description: Import your existing Terraform Setup to Terramate
---

# Start with existing Terraform Projects

## Import Existing Terraform Stacks

To create Terramate Stacks from existing Terraform Root Modules run the following command.

```bash
terramate create --all-terraform
```

This command will detect existing Terraform Root modules and create a stack configuration in them.

## Terramate Features for Terraform Repositories

All Terramate features are now available to your team.

The following set of features highlights some special benefits:

- Use Terramate Change Detection to orchestrate Terraform in an efficient way
- Execute **any** command within stacks imported from terraform configuration.
- Run Terraform in any CI/CD following the Terramate Automation Blueprints and examples.
- Make use of Terramates advanced Code Generation and Globals to share data more easily.
- Synchronize deployments, drift runs, and previews to **Terramate Cloud** and get
  - Visibility of the Health of all Terraform Configurations over multiple repositories
  - Drift Detection in all Stacks
  - Pull Request Previews for actual changes
  - Notifications on deployment failures or newly detected drifts
  - Advanced collaboration and alert routing

## Run Terraform Commands

### List all Stacks

Any Terramate CLI Feature is now available in your Stacks.

```bash
terramate list
```

### Init Terraform

```bash
terramate run -- terraform init
```

### Create a Terraform Plan in parallel

```bash
terramate run --parallel 5 -- terraform plan -out plan.tfplan
```

### Apply a Terraform Plan in Changed Stacks

```bash
terramate run --changed -- terraform apply -out plan.tfplan -auto-approve
```
