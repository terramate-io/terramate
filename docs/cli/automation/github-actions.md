---
title: How to use Terramate to automate and orchestrate Terraform in GitHub Actions
description: Learn how to use Terramate to configure custom GitOps workflows to automate and orchestrate Terraform and OpenTofu in GitHub Actions.
---

# Automating Terramate in GitHub Actions

GitHub Actions add continuous integration to GitHub repositories to automate your software builds, tests, and deployments. Automating Terraform with CI/CD enforces configuration best practices, promotes collaboration, and automates the Terraform workflow.


Terramate integrates seamlessly with GitHub Actions to automate and orchestrate IaC tools such as Terraform and OpenTofu.
The following GitHub Action workflows provide 

::: tip
You can find a reference architecture to get started with Terramate, Terraform, AWS and GitHub Actions in no time
at [terramate-quickstart-aws](https://github.com/terramate-io/terramate-quickstart-aws).
:::

The :

- Run a plan for each changed stack within a Pull Request and append those as comments to enable users to review changes in Pull Requests.
- Run apply on changed stacks when merging the Pull Request to the `main` branch.

```yml
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

    steps:

      # ## Create Pull Request comment

      - name: Prepare pull request preview comment
        if: github.event.pull_request
        uses: marocchino/sticky-pull-request-comment@v2
        with:
          GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}
          header: preview
          message: |
            ## Preview of Terraform changes in ${{ github.event.pull_request.head.sha }}

            :warning: preview is being created... please stand by!

      # ## Check out the code

      - name: Checkout
        uses: actions/checkout@v4
        with:
          ref: ${{ github.head_ref }}
          fetch-depth: 0

      # ## Install tooling

      - name: Install Terramate
        uses: terramate-io/terramate-action@v1

      - name: Install Terraform
        uses: hashicorp/setup-terraform@v3
        with:
          terraform_version: 1.7.4

      # ## Linting

      - name: Check Terramate formatting
        run: terramate fmt --check

      - name: Check Terraform formatting
        run: terraform fmt -recursive -check -diff

      # ## Check for changed stacks

      - name: List changed stacks
        id: list
        run: terramate list --changed

      # ## Configure cloud credentials

      # - name: Configure AWS credentials
      #   if: steps.list.outputs.stdout
      #   uses: aws-actions/configure-aws-credentials@v2
      #   with:
      #     aws-region: ${{ env.AWS_REGION }}
      #     role-to-assume: arn:aws:iam::${{ env.AWS_ACCOUNT_ID }}:role/github
      #   env:
      #     AWS_REGION: us-east-1
      #     AWS_ACCOUNT_ID: 0123456789
      #
      # - name: Verify AWS credentials
      #   if: steps.list.outputs.stdout
      #   run: aws sts get-caller-identity

      # ## Run the Terraform preview via Terramate in each changed stack

      - name: Initialize Terraform in changed stacks
        if: steps.list.outputs.stdout
        run: terramate run --changed -- terraform init -lock-timeout=5m

      - name: Validate Terraform configuration in changed stacks
        if: steps.list.outputs.stdout
        run: terramate run --changed -- terraform validate

      - name: Plan Terraform changes in changed stacks
        if: steps.list.outputs.stdout
        run: terramate run --changed -- terraform plan -lock-timeout=5m -out out.tfplan

      # ## Update Pull Request comment

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
```

```yml
name: Terraform Deployment

on:
  push:
    branches:
      - main

jobs:
  deploy:
    name: Deploy Terraform changes in changed Terramate stacks

    permissions:
      id-token: write
      contents: read
      pull-requests: read

    runs-on: ubuntu-latest

    steps:
      # ## Check out the code

      - name: Checkout
        uses: actions/checkout@v4
        with:
          fetch-depth: 0

      # ## Install tooling

      - name: Install Terramate
        uses: terramate-io/terramate-action@v1

      - name: Install Terraform
        uses: hashicorp/setup-terraform@v3
        with:
          terraform_version: 1.7.4

      # ## Check for changed stacks

      - name: List changed stacks
        id: list
        run: terramate list --changed

      # ## Configure cloud credentials

      # - name: Configure AWS credentials
      #   if: steps.list.outputs.stdout
      #   uses: aws-actions/configure-aws-credentials@v2
      #   with:
      #     aws-region: ${{ env.AWS_REGION }}
      #     role-to-assume: arn:aws:iam::${{ env.AWS_ACCOUNT_ID }}:role/github
      #   env:
      #     AWS_REGION: us-east-1
      #     AWS_ACCOUNT_ID: 0123456789
      #
      # - name: Verify AWS credentials
      #   if: steps.list.outputs.stdout
      #   run: aws sts get-caller-identity

      # ## Run the Terraform deployment via Terramate in each changed stack

      - name: Run Terraform init on changed stacks
        if: steps.list.outputs.stdout
        id: init
        run: |
          terramate run --changed -- terraform init

      - name: Apply changes on changed stacks
        id: apply
        if: steps.list.outputs.stdout
        run: terramate run --changed  -- terraform apply -input=false -auto-approve -lock-timeout=5m
        env:
          GITHUB_TOKEN: ${{ github.token }}
```
