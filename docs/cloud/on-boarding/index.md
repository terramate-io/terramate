---
title: Start using Terramate Cloud
description: Learn how to connect Terramate CLI with Terramate Cloud and start synchronizing data for running drift detection, track deployments and create previews for pull requests.
---

# Connect Terramate CLI to Terramate Cloud

After installing Terramate CLI, setting up your repository, and importing any existing configuration you can start using Terramate Cloud. The onboarding should not take long but includes the time it needs to run your first drift check over all your stacks and requires some configuration of automation pipelines - But we have you covered with our existing Terramate Automation Blueprints for the most common CI/CD platforms.

## Create a Cloud Account

To use Terramate Cloud, you will need to sign up and create an organization on Terramate Cloud.

- Sign in and Sign up to [cloud.terramate.io](https://cloud.terramate.io) and
- Create your Organization

Remember the organization's short name that you set for accessing the organization on Terramate Cloud to configure your Terramate CLI in the next steps.

## Configure your repository

After creating your organization configure your Terramate Repository to define which Terramate Cloud organization to synchronize data to.

```hcl
terramate {
  config {
    cloud {
      organization = "{organization-short-name}"
    }
  }
}
```

## Login from CLI

To synchronize data from your local machine, you will need to `login` to Terramate Cloud from the CLI.
Terramate CLI will store a session on your machine after a successful login.

Use the following command to initiate the login.

```bash
terramate cloud login
```

After execution of this command, a browser window will open and you can sign in to terramate cloud.
When this login is successful a message to continue in the CLI is shown in the browser and the CLI will also confirm the success.

## Required permissions

To run one of the following commands you will need to have the following permissions:

- Read/Write access to provisioned cloud resources in AWS, Google Cloud, Azure, etc. is required to create the drift plan.
- Read/Write access to the terraform state files
- To gather metadata from GitHub about the pull request associated with the preview, a `GITHUB_TOKEN` needs to be exposed or a valid GitHub CLI configuration needs to be available.

Drift Runs and Previews can run with read-only permissions to cloud resources and state files, while deployments will need read/write access to run deployments like `terraform apply`

Recommendations when running in automation:

- Restrict elevated access to your cloud providers (AWS, Google Cloud, or Azure) and access to Terraform State to automation flows. The team does not need to have elevated access to create drift checks, previews or to run deployments.

- Use OpenID Connect (OIDC) to authenticate to your Cloud Provider to use short-lived credentials - Terramate CLI uses OIDC by default.

## Run an initial Drift Check

To easily synchronize all your stacks to Terramate Cloud a Drift Run is the first recommended step.
When synchronizing data to the cloud, Terramate will also synchronize the stacks that were affected by the action.
As a Drift Run is supposed to run over all stacks, all stacks will be visible after a successful Drift Run.

In addition, you will have visibility of the stacks that have drifts detected and can also see the details of the drifted resources.

### Use Terraform

```bash
terramate run \
  --cloud-sync-drift-status \
  --cloud-sync-terraform-plan-file=drift.tfplan \
  --continue-on-error \
  -- \
  terraform plan -detailed-exitcode -out drift.tfplan
```

### Use Terragrunt

```bash
terramate run \
  --cloud-sync-drift-status \
  --cloud-sync-terraform-plan-file=drift.tfplan \
  --continue-on-error \
  --terragrunt \
  -- \
  terragrunt plan -detailed-exitcode -out drift.tfplan
```

## Enable Slack Notifications

After running the initial drift run, Slacks Notifications shall be enabled to get notified about newly detected drifts.

This can also be done before the initial drift check but will lead to one notification per drifted stack in your channel and can be overwhelming depending on how many drifts are detected.

Slack Notifications for your organization can be enabled in the "Manage Organization" area in the "General" Section.

## Setup Automation

Doing the initial drift check manually is a good start but enabling data synchronization from automation will boost the visibility and guarantee continuous observability of actions in your IaC repositories.

Multiple git repositories from multiple owners can be connected to the same Terramate Organization.

It is recommended to set up automation for scheduled drift runs, and deployments and for generating previews for pull requests.

### Synchronize Drift Runs

Terramate CLI Drift Checks can run in any CI/CD and we provide Blueprints for various CI/CD platforms:

- [GitHub Actions Blueprints](../../cli/automation/github-actions/drift-check-workflow.md)
- GitLab CI Blueprints 🚧
- Bitbucket Pipelines Blueprints 🚧
- Azure DevOps Blueprints 🚧

Recommendations when synchronizing drift checks:

- Run a Drift Check at least every 24 hours to get a detailed history of when drifts were introduced.
- Run a Drift Check right after deployment and synchronize the result to Terramate Cloud to get an immediate health check and ensure the deployment is stable.
- Ensure that all drift checks run on all stacks even if some errors are detected using the `--continue-on-error` command line option
- Set up notifications to get informed about newly detected drifts in stacks

### Synchronize Deployments

Terramate CLI Deployments can run in any CI/CD and we provide Blueprints for various CI/CD platforms:

- [GitHub Actions Blueprints](../../cli/automation/github-actions/deployment-workflow.md)
- GitLab CI Blueprints 🚧
- Bitbucket Pipelines Blueprints 🚧
- Azure DevOps Blueprints 🚧

Recommendations when synchronizing deployments:

- Ensure that all deployments to all environments are synchronized to Terramate Cloud to have access to historic deployment data and to get notifications about failures when they happen
- Run a Drift Check right after the deployment and synchronize the result to Terramate Cloud to get an immediate health check and ensure the deployment is stable.
- Set up Notifications to get informed about new deployments and detect failures fast.

### Synchronize Previews

Terramate CLI Previews can run in any CI/CD and we provide Blueprints for various CI/CD platforms:

- [GitHub Actions Blueprints](../../cli/automation/github-actions/preview-workflow.md)
- GitLab CI Blueprints 🚧
- Bitbucket Pipelines Blueprints 🚧
- Azure DevOps Blueprints 🚧

## Next Steps

- Reconcile any detected drifts
- Learn about Terramate Cloud in this documentation
- Start using Terramate Cloud to
  - Get visibility and notifications about drifts when they happen.
  - Catch failed deployments when they happen and notify your team.
  - Gain visibility of overall stack health for all your repositories in a central place.
  - Share details about previews, drifts, and deployments with your team without the need to grant them elevated access to cloud resources.
- Share your feedback with us and join our roadmap to shape the future of the product as you need it.
