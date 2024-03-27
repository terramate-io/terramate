---
title: "On-boarding: Terragrunt On-boarding"
description: Import your existing Terragrunt Setup to Terramate
---

# Start with existing Terragrunt Projects

::: warning
This is an experimental command and is likely subject to change in the future.
It needs to be enabled in the experiments config by adding `terragrunt` to `terramate.config.experiments` list (see below) to make use of advanced features like Change Detection for Terragrunt.
:::

## Import Existing Terragrunt Stacks

To create Terramate Stacks from existing Terragrunt Modules run the following command.

```bash
terramate create --all-terragrunt
```

This command will detect existing Terragrunt Modules, create a stack configuration in them and will set up the order of execution in `before` and `after` attributes for detected Terragrunt dependencies.

## Terramate Features for Terrgrunt Repositories

All Terramate features are now available to your team, so you get the best of both worlds.

The following set of features highlights some special benefits:

- Use Terramate Change Detection to reduce run times of terragrunt commands
- Execute **any** command within stacks imported from terragrunt config.
- Run Terragrunt in any CI/CD following the Terramate Automation Blueprints and examples.
- Make use of Terramates advanced Code Generation and Globals to share data more easily.
- Use Terragrunt and plain Terraform side-by-side.
- Synchronize deployments, drift runs, and previews to **Terramate Cloud** and get
  - Visibility of the Health of all Terragrunt Modules over multiple repositories
  - Drift Detection in all Stacks
  - Pull Request Previews for actual changes
  - Notifications on deployment failures or newly detected drifts
  - Advanced collaboration and alert routing

## Run Terragrunt Commands

Since you are using Terramate to orchestrate Terragrunt now, the `terragrunt run-all` command is not needed anymore and you can replace it with `terramate run -- terragrunt <cmd>` to execute terragrunt within single stacks.

The main benefit you get is to be able to make use of Terramates advanced Change Detection and other filters to orchestrate Terragrunt execution and also any other tooling inside of the stacks.

### List all Stacks

After importing Stacks imported from Terragrunt are not special compared to other stacks.
Any Terramate CLI Feature is now available in those Stacks and you can run any commands within the stacks.

```bash
terramate list
```

### Init Terraform with Terragrunt

```bash
terramate run -- terragrunt init
```

### Create a Terraform Plan with Terragrunt in parallel

```bash
terramate run --parallel 5 -- terragrunt plan -out plan.tfplan
```

### Apply a Terraform Plan with Terragrunt in Changed Stacks

```bash
terramate run --changed -- terragrunt apply -out plan.tfplan -auto-approve
```

## Enable the Experiment

To enable the experiment add the following to any top-level terramate configuration like `terramate.tm.hcl`:

```hcl
terramate {
  config {
    experiments = ["terragrunt"]
  }
}
```
