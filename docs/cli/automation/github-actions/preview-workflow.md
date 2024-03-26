---
title: How to use Terramate to automate and orchestrate Terraform Previews in GitHub Actions
description: Learn how to use Terramate to configure custom GitOps workflows to automate and orchestrate Terraform and OpenTofu Previews in GitHub Actions.
---

# Run a Preview in GitHub Actions

The following workflows are blueprints and need some adjustments to work for you.

Search for `CHANGEME` to adjust needed credentials details for AWS and Google Cloud examples.

## Terramate Cloud support

When synchronizing previews to Terramate Cloud previews will be synchronized with a detailed status and can be reviewed on a stack level.

The following actions also post back previews as GitHub comments. This will soon be taken care of by Terramate Cloud to fix some short-comings:

- GitHub limits the size of a comment, so when creating large plans of many stacks, previews are truncated.
- Rate limits in GitHub sometimes prevent comments from being created.
- Terraform plans are hard to read and include a lot of unneeded information for the reviewer
- Destructive operations are hard to spot.

Terramate Cloud will provide:

- Readable plan previews
- Policies to set up blocking pull requests until admins review destructive operations
- Detection and blocking on required re-plan of a stale plan, when the stack was changed on the default branch and in a pull request and the code is not up to date.

## Deployment Blueprints

Create the following GitHub Actions configuration at `.github/workflows/preview.yml`

::: code-group

````yml [ AWS + Terramate Cloud ]
name: Terraform Preview

on:
  pull_request:
    branches:
      - main

jobs:
  preview:
    name: Plan Terraform changes in changed Terramate stacks
    runs-on: ubuntu-latest

    permissions:
      id-token: write
      contents: read
      pull-requests: write
      checks: read

    steps:
      ### Create Pull Request comment

      - name: Prepare pull request preview comment
        if: github.event.pull_request
        uses: marocchino/sticky-pull-request-comment@v2
        with:
          GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}
          header: preview
          message: |
            ## Preview of Terraform changes in ${{ github.event.pull_request.head.sha }}

            :warning: preview is being created... please stand by!

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

      ### Linting

      - name: Check Terramate formatting
        run: terramate fmt --check

      - name: Check Terraform formatting
        run: terraform fmt -recursive -check -diff

      ### Check for changed stacks

      - name: List changed stacks
        id: list
        run: terramate list --changed

      ## Configure cloud credentials

      - name: Configure AWS credentials via OIDC
        if: steps.list.outputs.stdout
        uses: aws-actions/configure-aws-credentials@v2
        with:
          aws-region: "CHANGEME: AWS REGION"
          role-to-assume: "CHANGEME: IAM ROLE ARN"

      ### Run the Terraform preview via Terramate in each changed stack

      - name: Initialize Terraform in changed stacks
        if: steps.list.outputs.stdout
        run: terramate run --parallel 1 --changed -- terraform init -lock-timeout=5m

      - name: Validate Terraform configuration in changed stacks
        if: steps.list.outputs.stdout
        run: terramate run --parallel 5 --changed -- terraform validate

      - name: Plan Terraform changes in changed stacks
        if: steps.list.outputs.stdout
        run: |
          terramate run --parallel 5 --changed --cloud-sync-preview --cloud-sync-terraform-plan-file=out.tfplan --debug-preview-url preview_url.txt --continue-on-error -- terraform plan -out out.tfplan -detailed-exitcode -lock=false
        env:
          GITHUB_TOKEN: ${{ github.token }}

      ### Update Pull Request comment

      - name: Generate preview details
        if: steps.list.outputs.stdout
        id: comment
        run: |
          echo >>pr-comment.txt "## Preview of Terraform changes in ${{ github.event.pull_request.head.sha }}"
          echo >>pr-comment.txt
          echo >>pr-comment.txt "[:mag: View Details on Terramate Cloud]($(cat preview_url.txt))"
          echo >>pr-comment.txt
          echo >>pr-comment.txt "### Changed Stacks"
          echo >>pr-comment.txt
          echo >>pr-comment.txt '```bash'
          echo >>pr-comment.txt "${{ steps.list.outputs.stdout }}"
          echo >>pr-comment.txt '```'
          echo >>pr-comment.txt
          echo >>pr-comment.txt "#### Terraform Plan"
          echo >>pr-comment.txt
          echo >>pr-comment.txt '```terraform'
          terramate run --changed -- terraform show -no-color out.tfplan |& dd bs=1024 count=248 >>pr-comment.txt
          [ "${PIPESTATUS[0]}" == "141" ] && sed -i 's/#### Terraform Plan/#### :warning: Terraform Plan truncated: please check console output :warning:/' pr-comment.txt
          echo >>pr-comment.txt '```'
          cat pr-comment.txt >>$GITHUB_STEP_SUMMARY

      - name: Generate preview when no stacks changed
        if: success() && !steps.list.outputs.stdout
        run: |
          echo >>pr-comment.txt "## Preview of Terraform changes in ${{ github.event.pull_request.head.sha }}"
          echo >>pr-comment.txt
          echo >>pr-comment.txt "### Changed Stacks"
          echo >>pr-comment.txt
          echo >>pr-comment.txt 'No changed stacks, no detailed preview will be generated.'
          cat pr-comment.txt >>$GITHUB_STEP_SUMMARY

      - name: Generate preview when things failed
        if: failure()
        run: |
          echo >>pr-comment.txt "## Preview of Terraform changes in ${{ github.event.pull_request.head.sha }}"
          echo >>pr-comment.txt
          echo >>pr-comment.txt "[:mag: View Details on Terramate Cloud]($(cat preview_url.txt))"
          echo >>pr-comment.txt
          echo >>pr-comment.txt "### Changed Stacks"
          echo >>pr-comment.txt
          echo >>pr-comment.txt '```bash'
          echo >>pr-comment.txt "${{ steps.list.outputs.stdout }}"
          echo >>pr-comment.txt '```'
          echo >>pr-comment.txt ':boom: Generating preview failed. Please see details in Actions output.'
          cat pr-comment.txt >>$GITHUB_STEP_SUMMARY

      - name: Publish generated preview as GitHub commnent
        uses: marocchino/sticky-pull-request-comment@v2
        with:
          GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}
          header: preview
          path: pr-comment.txt
````

````yml [ GCP + Terramate Cloud ]
name: Terraform Preview

on:
  pull_request:
    branches:
      - main

jobs:
  preview:
    name: Plan Terraform changes in changed Terramate stacks
    runs-on: ubuntu-latest

    permissions:
      id-token: write
      contents: read
      pull-requests: write
      checks: read

    steps:
      ### Create Pull Request comment

      - name: Prepare pull request preview comment
        if: github.event.pull_request
        uses: marocchino/sticky-pull-request-comment@v2
        with:
          GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}
          header: preview
          message: |
            ## Preview of Terraform changes in ${{ github.event.pull_request.head.sha }}

            :warning: preview is being created... please stand by!

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

      ### Linting

      - name: Check Terramate formatting
        run: terramate fmt --check

      - name: Check Terraform formatting
        run: terraform fmt -recursive -check -diff

      ### Check for changed stacks

      - name: List changed stacks
        id: list
        run: terramate list --changed

      ## Configure cloud credentials

      - name: Authenticate to Google Cloud via OIDC
        if: steps.list.outputs.stdout
        id: auth
        uses: google-github-actions/auth@v1
        with:
          workload_identity_provider: "CHANGEME: WORKLOAD IDENTITY PROVIDER ID"
          service_account: "CHANGEME: SERVICE ACCOUNT EMAIL"

      ### Run the Terraform preview via Terramate in each changed stack

      - name: Initialize Terraform in changed stacks
        if: steps.list.outputs.stdout
        run: terramate run --parallel 1 --changed -- terraform init -lock-timeout=5m

      - name: Validate Terraform configuration in changed stacks
        if: steps.list.outputs.stdout
        run: terramate run --parallel 5 --changed -- terraform validate

      - name: Plan Terraform changes in changed stacks
        if: steps.list.outputs.stdout
        run: |
          terramate run --parallel 5 --changed --cloud-sync-preview --cloud-sync-terraform-plan-file=out.tfplan --debug-preview-url preview_url.txt --continue-on-error -- terraform plan -out out.tfplan -detailed-exitcode -lock=false
        env:
          GITHUB_TOKEN: ${{ github.token }}

      ### Update Pull Request comment

      - name: Generate preview details
        if: steps.list.outputs.stdout
        id: comment
        run: |
          echo >>pr-comment.txt "## Preview of Terraform changes in ${{ github.event.pull_request.head.sha }}"
          echo >>pr-comment.txt
          echo >>pr-comment.txt "[:mag: View Details on Terramate Cloud]($(cat stacks/preview_url.txt))"
          echo >>pr-comment.txt
          echo >>pr-comment.txt "### Changed Stacks"
          echo >>pr-comment.txt
          echo >>pr-comment.txt '```bash'
          echo >>pr-comment.txt "${{ steps.list.outputs.stdout }}"
          echo >>pr-comment.txt '```'
          echo >>pr-comment.txt
          echo >>pr-comment.txt "#### Terraform Plan"
          echo >>pr-comment.txt
          echo >>pr-comment.txt '```terraform'
          terramate run --changed -- terraform show -no-color out.tfplan |& dd bs=1024 count=248 >>pr-comment.txt
          [ "${PIPESTATUS[0]}" == "141" ] && sed -i 's/#### Terraform Plan/#### :warning: Terraform Plan truncated: please check console output :warning:/' pr-comment.txt
          echo >>pr-comment.txt '```'
          cat pr-comment.txt >>$GITHUB_STEP_SUMMARY

      - name: Generate preview when no stacks changed
        if: success() && !steps.list.outputs.stdout
        run: |
          echo >>pr-comment.txt "## Preview of Terraform changes in ${{ github.event.pull_request.head.sha }}"
          echo >>pr-comment.txt
          echo >>pr-comment.txt "### Changed Stacks"
          echo >>pr-comment.txt
          echo >>pr-comment.txt 'No changed stacks, no detailed preview will be generated.'
          cat pr-comment.txt >>$GITHUB_STEP_SUMMARY

      - name: Generate preview when things failed
        if: failure()
        run: |
          echo >>pr-comment.txt "## Preview of Terraform changes in ${{ github.event.pull_request.head.sha }}"
          echo >>pr-comment.txt
          echo >>pr-comment.txt "[:mag: View Details on Terramate Cloud]($(cat stacks/preview_url.txt))"
          echo >>pr-comment.txt
          echo >>pr-comment.txt "### Changed Stacks"
          echo >>pr-comment.txt
          echo >>pr-comment.txt '```bash'
          echo >>pr-comment.txt "${{ steps.list.outputs.stdout }}"
          echo >>pr-comment.txt '```'
          echo >>pr-comment.txt ':boom: Generating preview failed. Please see details in Actions output.'
          cat pr-comment.txt >>$GITHUB_STEP_SUMMARY

      - name: Publish generated preview as GitHub commnent
        uses: marocchino/sticky-pull-request-comment@v2
        with:
          GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}
          header: preview
          path: pr-comment.txt
````

````yml [ AWS ]
name: Terraform Preview

on:
  pull_request:
    branches:
      - main

jobs:
  preview:
    name: Plan Terraform changes in changed Terramate stacks
    runs-on: ubuntu-latest

    permissions:
      id-token: write
      contents: read
      pull-requests: write
      checks: read

    steps:
      ### Create Pull Request comment

      - name: Prepare pull request preview comment
        if: github.event.pull_request
        uses: marocchino/sticky-pull-request-comment@v2
        with:
          GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}
          header: preview
          message: |
            ## Preview of Terraform changes in ${{ github.event.pull_request.head.sha }}

            :warning: preview is being created... please stand by!

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

      ### Linting

      - name: Check Terramate formatting
        run: terramate fmt --check

      - name: Check Terraform formatting
        run: terraform fmt -recursive -check -diff

      ### Check for changed stacks

      - name: List changed stacks
        id: list
        run: terramate list --changed

      ## Configure cloud credentials

      - name: Configure AWS credentials via OIDC
        if: steps.list.outputs.stdout
        uses: aws-actions/configure-aws-credentials@v2
        with:
          aws-region: "CHANGEME: AWS REGION"
          role-to-assume: "CHANGEME: IAM ROLE ARN"

      ### Run the Terraform preview via Terramate in each changed stack

      - name: Initialize Terraform in changed stacks
        if: steps.list.outputs.stdout
        run: terramate run --parallel 1 --changed -- terraform init -lock-timeout=5m

      - name: Validate Terraform configuration in changed stacks
        if: steps.list.outputs.stdout
        run: terramate run --parallel 5 --changed -- terraform validate

      - name: Plan Terraform changes in changed stacks
        if: steps.list.outputs.stdout
        run: |
          terramate run --parallel 5 --changed --continue-on-error -- terraform plan -out out.tfplan -detailed-exitcode -lock=false
        env:
          GITHUB_TOKEN: ${{ github.token }}

      ### Update Pull Request comment

      - name: Generate preview details
        if: steps.list.outputs.stdout
        id: comment
        run: |
          echo >>pr-comment.txt "## Preview of Terraform changes in ${{ github.event.pull_request.head.sha }}"
          echo >>pr-comment.txt
          echo >>pr-comment.txt "### Changed Stacks"
          echo >>pr-comment.txt
          echo >>pr-comment.txt '```bash'
          echo >>pr-comment.txt "${{ steps.list.outputs.stdout }}"
          echo >>pr-comment.txt '```'
          echo >>pr-comment.txt
          echo >>pr-comment.txt "#### Terraform Plan"
          echo >>pr-comment.txt
          echo >>pr-comment.txt '```terraform'
          terramate run --changed -- terraform show -no-color out.tfplan |& dd bs=1024 count=248 >>pr-comment.txt
          [ "${PIPESTATUS[0]}" == "141" ] && sed -i 's/#### Terraform Plan/#### :warning: Terraform Plan truncated: please check console output :warning:/' pr-comment.txt
          echo >>pr-comment.txt '```'
          cat pr-comment.txt >>$GITHUB_STEP_SUMMARY

      - name: Generate preview when no stacks changed
        if: success() && !steps.list.outputs.stdout
        run: |
          echo >>pr-comment.txt "## Preview of Terraform changes in ${{ github.event.pull_request.head.sha }}"
          echo >>pr-comment.txt
          echo >>pr-comment.txt "### Changed Stacks"
          echo >>pr-comment.txt
          echo >>pr-comment.txt 'No changed stacks, no detailed preview will be generated.'
          cat pr-comment.txt >>$GITHUB_STEP_SUMMARY

      - name: Generate preview when things failed
        if: failure()
        run: |
          echo >>pr-comment.txt "## Preview of Terraform changes in ${{ github.event.pull_request.head.sha }}"
          echo >>pr-comment.txt
          echo >>pr-comment.txt "### Changed Stacks"
          echo >>pr-comment.txt
          echo >>pr-comment.txt '```bash'
          echo >>pr-comment.txt "${{ steps.list.outputs.stdout }}"
          echo >>pr-comment.txt '```'
          echo >>pr-comment.txt ':boom: Generating preview failed. Please see details in Actions output.'
          cat pr-comment.txt >>$GITHUB_STEP_SUMMARY

      - name: Publish generated preview as GitHub commnent
        uses: marocchino/sticky-pull-request-comment@v2
        with:
          GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}
          header: preview
          path: pr-comment.txt
````

````yml [ GCP ]
name: Terraform Preview

on:
  pull_request:
    branches:
      - main

jobs:
  preview:
    name: Plan Terraform changes in changed Terramate stacks
    runs-on: ubuntu-latest

    permissions:
      id-token: write
      contents: read
      pull-requests: write
      checks: read

    steps:
      ### Create Pull Request comment

      - name: Prepare pull request preview comment
        if: github.event.pull_request
        uses: marocchino/sticky-pull-request-comment@v2
        with:
          GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}
          header: preview
          message: |
            ## Preview of Terraform changes in ${{ github.event.pull_request.head.sha }}

            :warning: preview is being created... please stand by!

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

      ### Linting

      - name: Check Terramate formatting
        run: terramate fmt --check

      - name: Check Terraform formatting
        run: terraform fmt -recursive -check -diff

      ### Check for changed stacks

      - name: List changed stacks
        id: list
        run: terramate list --changed

      ## Configure cloud credentials

      - name: Authenticate to Google Cloud via OIDC
        if: steps.list.outputs.stdout
        id: auth
        uses: google-github-actions/auth@v1
        with:
          workload_identity_provider: "CHANGEME: WORKLOAD IDENTITY PROVIDER ID"
          service_account: "CHANGEME: SERVICE ACCOUNT EMAIL"

      ### Run the Terraform preview via Terramate in each changed stack

      - name: Initialize Terraform in changed stacks
        if: steps.list.outputs.stdout
        run: terramate run --parallel 1 --changed -- terraform init -lock-timeout=5m

      - name: Validate Terraform configuration in changed stacks
        if: steps.list.outputs.stdout
        run: terramate run --parallel 5 --changed -- terraform validate

      - name: Plan Terraform changes in changed stacks
        if: steps.list.outputs.stdout
        run: |
          terramate run --parallel 5 --changed --continue-on-error -- terraform plan -out out.tfplan -detailed-exitcode -lock=false
        env:
          GITHUB_TOKEN: ${{ github.token }}

      ### Update Pull Request comment

      - name: Generate preview details
        if: steps.list.outputs.stdout
        id: comment
        run: |
          echo >>pr-comment.txt "## Preview of Terraform changes in ${{ github.event.pull_request.head.sha }}"
          echo >>pr-comment.txt
          echo >>pr-comment.txt "### Changed Stacks"
          echo >>pr-comment.txt
          echo >>pr-comment.txt '```bash'
          echo >>pr-comment.txt "${{ steps.list.outputs.stdout }}"
          echo >>pr-comment.txt '```'
          echo >>pr-comment.txt
          echo >>pr-comment.txt "#### Terraform Plan"
          echo >>pr-comment.txt
          echo >>pr-comment.txt '```terraform'
          terramate run --changed -- terraform show -no-color out.tfplan |& dd bs=1024 count=248 >>pr-comment.txt
          [ "${PIPESTATUS[0]}" == "141" ] && sed -i 's/#### Terraform Plan/#### :warning: Terraform Plan truncated: please check console output :warning:/' pr-comment.txt
          echo >>pr-comment.txt '```'
          cat pr-comment.txt >>$GITHUB_STEP_SUMMARY

      - name: Generate preview when no stacks changed
        if: success() && !steps.list.outputs.stdout
        run: |
          echo >>pr-comment.txt "## Preview of Terraform changes in ${{ github.event.pull_request.head.sha }}"
          echo >>pr-comment.txt
          echo >>pr-comment.txt "### Changed Stacks"
          echo >>pr-comment.txt
          echo >>pr-comment.txt 'No changed stacks, no detailed preview will be generated.'
          cat pr-comment.txt >>$GITHUB_STEP_SUMMARY

      - name: Generate preview when things failed
        if: failure()
        run: |
          echo >>pr-comment.txt "## Preview of Terraform changes in ${{ github.event.pull_request.head.sha }}"
          echo >>pr-comment.txt
          echo >>pr-comment.txt "### Changed Stacks"
          echo >>pr-comment.txt
          echo >>pr-comment.txt '```bash'
          echo >>pr-comment.txt "${{ steps.list.outputs.stdout }}"
          echo >>pr-comment.txt '```'
          echo >>pr-comment.txt ':boom: Generating preview failed. Please see details in Actions output.'
          cat pr-comment.txt >>$GITHUB_STEP_SUMMARY

      - name: Publish generated preview as GitHub commnent
        uses: marocchino/sticky-pull-request-comment@v2
        with:
          GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}
          header: preview
          path: pr-comment.txt
````

:::
