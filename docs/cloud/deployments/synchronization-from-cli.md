---
title: Synchronize Deployments from CLI
description: Learn how to synchronize deployment status, logs and details with Terramate CLI to Terramate Cloud.
---

# Synchronize Deployments from CLI

To display deployments on Terramate Cloud, we need to synchronize the status and details.

When already using [Terramate CLI](../../introduction.md) to orchestrate your stacks, the configuration needed to synchronize deployments is minimal.

You can synchronize deployments using [`terramate run`](../../cli/cmdline/run.md) or reduce the overhead on the caller
side by using [Terramate Scripts](../../cli/orchestration/scripts.md), e.g. `terramate script run` where you can trigger
deployment sync automatically.

## Required Permission

To run the command on the local machine `terramate cloud login` needs to be executed before.
When run in CI/CD, Terramate CLI will pick up the OpenID Connect (OIDC) tokens and authenticate to the cloud.

To gather metadata from GitHub about the pull request associated with the preview, a `GITHUB_TOKEN` needs to be exposed or a valid GitHub CLI configuration needs to be available.

## `terramate run`

The [run](../../cli/cmdline/run.md) command in Terramate CLI offers some command line options to synchronize deployment
information to Terramate Cloud.

- `--cloud-sync-deployment` Synchronizes the deployment status and logs to Terramate Cloud
- `--cloud-sync-terraform-plan FILE` A Terraform integration that allows synchronization of details about the changed, added, or deleted Terraform Resources that were planned to be applied in the deployment

The full command line to run an auto-approved apply based on an existing plan file in changed stacks only looks like the following:

```bash
terramate run \
  --cloud-sync-deployment \
  --cloud-sync-terraform-plan out.tfplan \
  --changed \
  terraform apply -auto-approve out.tfplan
```

::: info
Make sure to use the same plan file for the `terraform apply` command and the Terraform Integration.
:::

It is recommended to create a Terramate Script as explained in the next section, to provide an easy interface for users
that can be used on local machines the same way as in CI/CD automated environments. This way the options do not need to
be added and memorized.

In case communication with Terramate Cloud fails, the deployments will continue as expected but the deployment will not
be synchronized with Terramate Cloud. Warning messages will help you identify any problems.
