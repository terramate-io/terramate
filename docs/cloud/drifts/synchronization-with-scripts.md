---
title: Synchronize Drift Checks via Scripts
description: Learn how to create a Terramate Script to synchronize drift status with Terramate CLI to Terramate Cloud in automation or from local machines.
---

# Synchronize Drift Checks via Scripts

The following Terramate Script is a template that can be used as a starting point for creating a unified execution flow when checking stacks for drifts in automation or via the CLI (manual).

It guarantees that the drift status of each stack is always synchronized to Terramate Cloud.

## Required Permission

To run the script on the local machine `terramate cloud login` needs to be executed before.
When run in CI/CD, Terramate CLI will pick up the OpenID Connect (OIDC) tokens and authenticate to the cloud.

To gather metadata from GitHub about the commit associated with the drift check, a `GITHUB_TOKEN` needs to be exposed or a valid GitHub CLI configuration needs to be available.

## Command Options

The following options are available in Terramate Scripts and mirror the CLI options with the name:

- Set `cloud_sync_drift_status = true` to let Terramate CLI know about the command that is doing the actual drift check and returns a detailed exit status to define a successful run that has changed or has no changes detected.
- Set `cloud_sync_terraform_plan_file` to the name of the terraform plan to synchronize the deployment details.
- Set `terragrunt = true` to use terragrunt for the plan file generation.

## Terramate Script Config

The script is executed with `terramate script run terraform detect-drift`.

```hcl
script "terraform" "detect-drift" {
  name        = "Terraform Drift Check"
  description = "Detect drifts in Terraform configuration and synchronize it to Terramate Cloud."

  job {
    name        = "Terraform Plan"
    description = "Initialize, validate, and plan Terraform changes."
    commands = [
      ["terraform", "init", "-lock-timeout=5m"],
      ["terraform", "plan", "-out", "drift.tfplan", "-detailed-exitcode", "-lock=false", {
        cloud_sync_drift_status        = true
        cloud_sync_terraform_plan_file = "drift.tfplan"
      }],
    ]
  }
}
```
