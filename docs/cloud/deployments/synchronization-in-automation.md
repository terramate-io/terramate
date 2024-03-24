---
title: Synchronize Deployments in Automation
description: Learn how to synchronize deployment status, logs and details with Terramate CLI to Terramate Cloud in automation.
---

# Synchronize Deployments in Automation

Automation is the recommended way to run deployments and synchronize the results to Terramate Cloud.

## Automation Blueprints

Terramate CLI can run in any CI/CD and we provide Blueprints for various CI/CD platforms:

- [GitHub Actions Blueprints](../../cli/automation/github-actions/deployment-workflow.md)
- GitLab CI Blueprints 🚧
- Bitbucket Pipelines Blueprints 🚧
- Azure DevOps Blueprints 🚧

## Required Permission

To gather metadata from GitHub about the pull request associated with the preview, a `GITHUB_TOKEN` needs to be exposed or a valid GitHub CLI configuration needs to be available.

## Best Practices

- Restrict elevated access to your cloud providers (AWS, Google Cloud, or Azure) and access to Terraform State to automation flows.
- Use OpenID Connect (OIDC) to authenticate to your Cloud Provider to use short-lived credentials - Terramate CLI uses OIDC by default.
- Ensure that all deployments to all environments are synchronized to Terramate Cloud to have access to historic deployment data and to get notifications about failures when they happen
- Run a Drift Check right after the deployment and synchronize the result to Terramate Cloud to get an immediate health check and ensure the deployment is stable.
- Set up Notifications to get informed about new deployments and detect failures fast.
