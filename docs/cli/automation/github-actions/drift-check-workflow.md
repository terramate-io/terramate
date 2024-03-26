---
title: How to use Terramate to automate and orchestrate Terraform Drift Checks in GitHub Actions
description: Learn how to use Terramate to configure custom GitOps workflows to automate and orchestrate Terraform and OpenTofu Drift Checks in GitHub Actions.
---

# Run a Drfit Check in GitHub Actions

The following workflows are blueprints and need some adjustments to work for you.

Search for `CHANGEME` to adjust needed credentials details for AWS and Google Cloud examples.

Drift Checks require action and protocolling the results, so Terramate Cloud support is required for those workflows at the moment.

The following workflows run every day at 2 am.

## Terramate Cloud support

When synchronizing drift checks to Terramate Cloud, the following features will support the team with handling drifts:

- Get notified on new drifts via Slack notifications.
- Highlight and identify drifted stacks in the Stacks List and Dashboard
- See drift details without requiring your team to have elevated access to read the Terraform state or have access to read the cloud resources.
- Identify the time when a drift happened and how long a stack stayed in a drifted state.
- Create automation to reconcile a drift without human interaction using the `--cloud-status` filter in Terramate CLI.

## Deployment Blueprints

Create the following GitHub Actions configuration at `.github/workflows/drift.yml`

::: code-group

```yml [ AWS + Terramate Cloud ]
name: Scheduled Terraform Drift Detection

on:
  schedule:
    - cron: "0 2 * * *"

jobs:
  drift-detection:
    name: Check Drift

    permissions:
      id-token: write
      contents: read
      pull-requests: read
      checks: read

    runs-on: ubuntu-latest

    steps:
      ### Check out the code

      - name: Checkout
        uses: actions/checkout@v4
        with:
          ref: ${{ github.head_ref }}
          fetch-depth: 0

      ## Install tooling

      - name: Install Terramate
        uses: terramate-io/terramate-action@v1

      - name: Install Terraform
        uses: hashicorp/setup-terraform@v3
        with:
          terraform_version: 1.7.4

      ## Configure cloud credentials

      - name: Configure AWS credentials via OIDC
        if: steps.list.outputs.stdout
        uses: aws-actions/configure-aws-credentials@v2
        with:
          aws-region: "CHANGEME: AWS REGION"
          role-to-assume: "CHANGEME: IAM ROLE ARN"

      ### Run Dift Check

      - name: Run Terraform init on all stacks
        id: init
        run: terramate run -C stacks -- terraform init

      - name: Run drift detection
        id: drift
        run: terramate run -C stacks --cloud-sync-drift-status --cloud-sync-terraform-plan-file=drift.tfplan --continue-on-error --parallel 5 -- terraform plan -out drift.tfplan -detailed-exitcode -lock=false
        env:
          GITHUB_TOKEN: ${{ github.token }}
```

```yml [ GCP + Terramate Cloud ]
name: Scheduled Terraform Drift Detection

on:
  schedule:
    - cron: "0 2 * * *"

jobs:
  drift-detection:
    name: Check Drift

    permissions:
      id-token: write
      contents: read
      pull-requests: read
      checks: read

    runs-on: ubuntu-latest

    steps:
      ### Check out the code

      - name: Checkout
        uses: actions/checkout@v4
        with:
          ref: ${{ github.head_ref }}
          fetch-depth: 0

      ## Install tooling

      - name: Install Terramate
        uses: terramate-io/terramate-action@v1

      - name: Install Terraform
        uses: hashicorp/setup-terraform@v3
        with:
          terraform_version: 1.7.4

      ## Configure cloud credentials

      - name: Authenticate to Google Cloud via OIDC
        if: steps.list.outputs.stdout
        id: auth
        uses: google-github-actions/auth@v1
        with:
          workload_identity_provider: "CHANGEME: WORKLOAD IDENTITY PROVIDER ID"
          service_account: "CHANGEME: SERVICE ACCOUNT EMAIL"

      ### Run Dift Check

      - name: Run Terraform init on all stacks
        id: init
        run: terramate run -C stacks -- terraform init

      - name: Run drift detection
        id: drift
        run: terramate run -C stacks --cloud-sync-drift-status --cloud-sync-terraform-plan-file=drift.tfplan --continue-on-error --parallel 5 -- terraform plan -out drift.tfplan -detailed-exitcode -lock=false
        env:
          GITHUB_TOKEN: ${{ github.token }}
```
:::
