---
title: How to use Terramate to automate and orchestrate Terraform in GitHub Actions
description: Learn how to use Terramate to configure custom GitOps workflows to automate and orchestrate Terraform and OpenTofu in GitHub Actions.
---

# Automating Terramate in GitHub Actions

GitHub Actions add continuous integration to GitHub repositories to automate your software builds, tests, and deployments. Automating Terraform with CI/CD enforces configuration best practices, promotes collaboration, and automates the Terraform workflow.

Terramate integrates seamlessly with GitHub Actions to automate and orchestrate IaC tools such as Terraform and OpenTofu.

::: tip
You can find a reference architecture to get started with Terramate, Terraform, AWS, and GitHub Actions in no time
at [terramate-quickstart-aws](https://github.com/terramate-io/terramate-quickstart-aws).
:::

## Terramate Blueprints

This page explains some details about the workflows, required permissions, and authentication flows that the following workflows have in common.

To jump directly into the Blueprints follow the links below:

- [Deployment Workflow Blueprints](./deployment-workflow.md)
- [Drift Check Workflow Blueprints](./drift-check-workflow.md)
- [Pull Request Preview Workflow Blueprints](./preview-workflow.md)

Please read the following sections to understand the details all those workflows have in common.

## Workflow Permissions

To be able to use password-less authentication to cloud providers as well as Terramate Cloud specific permissions are needed on the GitHub Token that is used to run the workflow.
In addition, when synchronizing to Terramate Cloud, `terramate` also synchronizes details about the GitHub environment.
For this process, Terramate needs additional permissions to read pull-request details and checks.

- `id-token: write` Allow to create an OIDC TOKEN for exchange with Cloud Credentials and to authenticate to Terramate Cloud
- `contents: read` Allow to check the code from the repository
- `pull-requests: read` Allow to read pull request details
- `checks: read` Allow to read workflow details

Terramate Cloud synchronization enables you to have visibility of the executed deployment and status and allows you to get notified via Slack when the deployments fails or a drift is detected in the health check step. For this, the `GITHUB_TOKEN` environment variables need to be exposed to the commands that synchronize to Terramate Cloud.

## Merge and Apply Strategy

The GitHub Actions Workflow Blueprints follow the merge+apply strategy, where deployments happen when a pull request is merged to the `main` branch.

## Code Checkout

For the Change Detection to work, the git history is needed to be able to compare the current commit with previous commits.
this is achieved by adding the `fetch-depth: 0` option to the `actions/checkout@v4` GitHub Action.

## Cloud Authentication

Search for `CHANGEME` to adjust needed credentials details for AWS and Google Cloud examples.
The workflows use OpenID Connect (OIDC) which is a password-less workflow and the recommended way to authenticate to AWS and Google Cloud.
The IAM Role on AWS or the Service Account on Google Cloud needs to be configured for this authentication to succeed.

- [Configuring OpenID Connect in Amazon Web Services](https://docs.github.com/en/actions/deployment/security-hardening-your-deployments/configuring-openid-connect-in-amazon-web-services)
- [Configuring OpenID Connect in Google Cloud](https://docs.github.com/en/actions/deployment/security-hardening-your-deployments/configuring-openid-connect-in-google-cloud-platform)
