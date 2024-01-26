# Synchronize Deployments to Terramate Cloud

## Synchronize Deployments with Terramate CLI

In order to display deployments on Terramate Cloud, we need to synchronize the status and details.

When already using [Terramate CLI](../../introduction.md) to orchestrate your stacks, the configuration needed to synchronize deployments is minimal.

You can synchronize deployments using [`terramate run`](../../cli/cmdline/run.md) or reduce the overhead on the caller
side by using [Terramate Scripts](../../cli/orchestration/scripts.md), e.g. `terramate script run` where you can trigger
deployment sync automatically.

### Synchronize Deployments using `terramate run`

The [run](../../cli/cmdline/run.md) command in Terramate CLI offers some command line options to synchronize deployment
information to Terramate Cloud.

- `--cloud-sync-deployment` Synchronizes the deployment status and logs to Terramate Cloud
- `--cloud-sync-terraform-plan=FILE` A Terraform integration that allows to synchronize details about the changed,
added or deleted Terraform Resources that were planned to be applied in the deployment

The full command line to run an auto approved apply based on an existing plan file looks like the following:

```bash
terramate run \
  --cloud-sync-deployment \
  --cloud-sync-terraform-plan=out.tfplan \
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

You will need to be signed in to Terramate Cloud from Terramate CLI to synchronize deployments. When executing the
command from GitHub Actions, ensure you have set up a trust relationship between Terramate Cloud and your GitHub organization.

See [GitHub Trust](../organization/settings.md#general-settings) for more information.
