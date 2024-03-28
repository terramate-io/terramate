---
title: terramate run - Command
description: Execute any commands in all stacks or in a filtered subset of stacks by using the `terramate run` command.
---

# Run any Commands in Stacks

## Overview

The `terramate run` command executes **any command** in all or a subset of stacks honoring
the defined [order of execution](../orchestration/index.md). Commands can be executed sequentially or in parallel.

When running commands you can filter for a specific subset of stacks such as:

- Stacks that have changed in the current branch or since the last merge (git-based filtering).
- Stacks with or without specific tags are set in configuration.
- Stacks in a subtree of the repository.
- Stacks with a specific health status as known by Terramate Cloud, e.g. `drifted` or `failed` stacks.

For details on how the change detection and order of execution works in Terramate please see:

- [Change Detection](../change-detection/index.md)
- [Orchestration](../orchestration/index.md)

## Examples

### Run in all stacks

When running `terramate run` without any options, the given command will be executed in all stacks reachable from the current workdir.

First, initialize Terraform in all stacks, then run `terraform apply` in all stacks.

```bash
terramate run terraform init
terramate run terraform apply
```

### Run in opposite order

To reverse the order of execution you can use `--reverse` option.

```bash
terramate run --reverse terraform destroy
```

### Run in a subtree

To select a subtree within a repository, the `--chdir` global option can be set.

```bash
terramate run --chdir stacks/aws -- terraform init
```

### Run in changed stacks

Stacks [containing changes](../change-detection/index.md) can be selected with the `--changed` flag.
This flag is also supported in `terramate list` which can be used to preview the affected stacks in advance.

```bash
terramate run --changed -- terraform init
```

### Run in tagged stacks

```bash
terramate run --tags k8s:prd -- kubectl diff
```

### TMC: Deploy changed Stacks

In order to track deployments and get notified about deployment status in Slack, you need to enable synchronization of the deployment status and details to Terramate Cloud (TMC).

The following example runs 5 stacks in parallel, selects only recently changed stacks and synchronizes the results to Terramate Cloud (TMC).

```bash
# initialize changed stacks
terramate run --changed --parallel 5 -- \
  terraform init

# create a preview plan
terramate run --changed --parallel 5 -- \
  terraform plan -lock-timeout=5m -out out.tfplan

# run the deployment
terramate run --changed --parallel 5 \
  --cloud-sync-deployment \
  --cloud-sync-terraform-plan-file=out.tfplan \
  -- \
  terraform apply -input=false -auto-approve -lock-timeout=5m out.tfplan
```

Deployments can also be executed the same way without synchronizing the details to Terramate Cloud.
In this case, Terramate Cloud offers the following benefits:

- Get notified in Slack and tag involved team members.
- Create an Alert and auto-assign ownership on deployment failure to keep the team responsible.
- Keep track of deployment history, frequency and stack health.
- Keep track of deployed resources and the historic evolution of your stacks.
- Share deployment logs even when not executed in CI/CD but on local machines.
- See deployment logs per stack instead of unreadable logs when running stacks in parallel.

### TMC: Detect Drifts

In order to detect drifts and get notified about detected drifts in Slack, you need to enable synchronization of drift status and details to Terramate Cloud (TMC):

```bash
# initialize all stacks
terramate run --parallel 5 -- \\
  terraform init

# run the actual drift detection
terramate run --parallel 5 \
  --cloud-sync-drift-status \
  --cloud-sync-terraform-plan-file=drift.tfplan \
  -- \
  terraform plan -out drift.tfplan -detailed-exitcode -lock=false
```

Drifts can also be executed the same way without synchronizing the details to Terramate Cloud.

In this case, Terramate Cloud offers the following benefits:

- Get notified in Slack when a new drift is detected in a stack.
- Create an Alert and auto-assign ownership on the detected drift to keep the team responsible.
- Keep track of drift history, frequency and stack health.
- Keep track of drifted resources and the historic evolution of your stacks.
- Never miss a drift again when running the drift detection on a schedule in your CI/CD automation.
- Automatic drift reconciliation (see the following example)
- Automatic healing of stacks when a drift gets resolved via a deployment or manual.

### TMC: Auto reconcile Drifts

To auto-reconcile drifts, three things are needed:

- **Drift Detection is run** regularly and detected drifts are synchronized to Terramate Cloud (TMC)
- **Stacks are tagged** to participate in auto reconciliation of drifts
  _(it is not recommended to reconcile just any stack due to the blast radius of potential destructive operations)_
- **Automation is set up** to run drift detection and reconciliation scheduled in automation like GitHub Actions.

The command can be tested locally by executing:

```bash
terramate run --cloud-status drifted --tags auto-reconcile-drift -- terraform apply
```

## Usage

```bash
terramate run [options] -- <cmd ...>
```

`[options]` can be one or multiple of the following options:

## Options

### Influence orchestration

- `--continue-on-error`

  Do not stop execution when an error occurs.

- `--no-recursive`

  Do not recurse into nested child stacks. If executed in a non-stack directory, no stacks will be run.

- `--dry-run`

  Plan the execution but do not execute it.

- `--reverse`

  Reverse the order of execution.

- `--parallel <n>`, `-j <n>`

  Run independent stacks in parallel.

- `--disable-safeguards <types>`, `-`X` A comma-separated list of safeguards.

  This option can be used multiple times.

  Disable safeguards. `-X` is short for disabling `all` safeguards.

  `<types>` can be one or multiple of:

  - `all` - Disable all safeguards. Use `-X` as a shorthand for this.
  - `none` - Enable all safeguards if disabled by config or environment variable.
  - `git` - Disable all `git`-based safeguards
    - `git-untracked` - Disable safeguarding against untracked files.
    - `git-uncommitted` - Disable safeguarding against uncommitted changes.
    - `git-out-of-sync` - Disable safeguarding against being out of sync with the remote default git branch.
  - `outdated-code` - Disable safeguarding against outdated code generation

  Safeguards can also be permanently or temporarily disabled via

  - Terramate Configuration `terramate.config.run.disable_safeguard = "<types>"`
  - Environment variable `TM_DISABLE_SAFEGUARDS=<types>`

### Interpolated command execution

- `--eval`

  Evaluate command arguments as HCL strings interpolating Globals, Functions and Metadata.

### Change detection support

- `--changed`, `-c`

  Filter stacks based on changes made in git.

  Example:

  ```bash
  terramate run --changed -- terraform init
  ```

- `--git-change-base <ref>`, `-B <ref>`

  Set git base reference for computing changes.

  Can only be used when change detection is enabled via the `--changed` option.

  Example:

  ```bash
  terramate run --changed --git-change-base HEAD~2 -- terraform init
  ```

### Filters for stacks

- `--tags <tags>`

  Filter stacks by tags.

- `--no-tags <tags>`

  Filter stacks by tags not being set.

## Terramate Cloud Options

### TMC: Advanced filters

- `--cloud-status <status>` is only available when connected to Terramate Cloud.

  Filter by Terramate Cloud (TMC) status of the stack.

### TMC: Deployment Synchronization

- `--cloud-sync-deployment` is only available when connected to Terramate Cloud.

  Synchronize the command as a new deployment to Terramate Cloud (TMC).

  For Terraform deployments `--cloud-sync-terraform-plan-file <plan-file>` should always be added to include Terraform plan details when synchronizing.

### TMC: Drift Synchronization

- `--cloud-sync-drift-status` is only available when connected to Terramate Cloud.

  Synchronize the command as a new drift run to Terramate Cloud (TMC).

  For Terraform drift runs `--cloud-sync-terraform-plan-file <plan-file>` should always be added to include Terraform plan details when synchronizing.

### TMC: Preview Synchronization

- `--cloud-sync-preview` is only available when connected to Terramate Cloud.

  Synchronize the command as a new preview to Terramate Cloud (TMC).

  For Terraform previews `--cloud-sync-terraform-plan-file <plan-file>` is required to include Terraform plan details when synchronizing.

- `--cloud-sync-layer <layer>`

  Default `<layer>` is `default` when not set or not detected otherwise.

  Set a custom layer for synchronizing a preview via `--cloud-sync-preview` to Terramate Cloud.

### TMC: Terraform Plan Synchronization

- `--cloud-sync-terraform-plan-file <plan-file>` is only available when connected to Terramate Cloud.

  Add details of the Terraform Plan file to the synchronization to Terramate Cloud (TMC).

  This flag is supported in combination with `--cloud-sync-drift-status`, `--cloud-sync-preview`, and `--cloud-sync-preview`.

- `--terragrunt`

  Use Terragrunt to generate the Terraform Plan file.

## Configuration of the Run Command

The `terramate` block at the project root can be used to customize
the default exported environment variables in the
[terramate.config.run.env](../projects/configuration.md#the-terramateconfigrunenv-block).

It's also possible to set a different `PATH` environment variable and
in this case, Terramate will honor it when looking up the program's
absolute path.

For example, let's say you have a `bin` directory at the root of the
Terramate project where you define some scripts that should be run in each stack.

In this case, you can have the declaration below in the root
directory:

```hcl
terramate {
  config {
    run {
      env {
        # prepend the bin/ directory so it has preference.
        PATH = "${terramate.root.path.fs.absolute}/bin:${env.PATH}"
      }
    }
  }
}
```

Then if you have the script `bin/deploy-terraform.sh`, you can do:

```bash
$ terramate run deploy-terraform.sh
```

## Terramate Cloud specifics

The run command offers extended functionality for Terramate Cloud users. For these commands to work `terramate` must be successfully authenticated with Terramate Cloud. This can be done locally with the [cloud login](./cloud/cloud-login.md) command or by creating a trust relationship with Github. In the latter case, you must export the `GITHUB_TOKEN` in the Github action:

```yaml
permissions:
  id-token: write # (necessary for GITHUB_TOKEN to work)
env:
  GITHUB_TOKEN: ${{ github.token }}
```

### Running a command on stacks with specific cloud status.

It's possible to run commands in stacks with specific cloud status.

For example, for applying all stacks with the `drifted` status, the command below
can be used:

```bash
$ terramate run --cloud-status=drifted -- terraform apply
```

Valid statuses are documented on the [trigger page](./experimental/experimental-trigger.md).

### Sending deployment data to Terramate Cloud

The `--cloud-sync-deployment` flag will send information about the deployment to Terramate Cloud.

```yaml
jobs:
  deploy:
    name: Deploy
    ...
      - name: Apply changes
        id: apply
        run: terramate run --changed --cloud-sync-deployment -- terraform apply -input=false -auto-approve
```

### Sending a pull request preview to Terramate Cloud

The `--cloud-sync-preview` flag will send information about the preview to Terramate Cloud.

```yaml
jobs:
  preview:
    name: Preview
    ...
      - name: Run preview
        id: preview
        run: |
          terramate run \
          --changed \
          --cloud-sync-preview \
          --cloud-sync-terraform-plan-file=preview.tfplan \
          -- \
          terraform plan -out preview.tfplan -detailed-exitcode
```

### Detecting Drift

The `run` command supports `--cloud-sync-drift-status` which will set the Terramate Cloud status of any stack to `drifted` _if the exit code of the command that is run is `2`_ (which for `terraform plan -detailed-exitcode` signals that the plan succeeded and there was a diff). Terramate is also able to send the drifted plan with the `--cloud-sync-terraform-plan-file` option. A typical Github action for drift detection would look something like this:

```yaml
name: Check drift on all stacks once a day

on:
  schedule:
    - cron: '0 2 * * *'

jobs:
  drift-detect:
    name: Check Drift

    permissions:
      id-token: write # necessary for GITHUB_TOKEN to work

    env:
      GITHUB_TOKEN: ${{ github.token }}

    steps:
      ...
      initial setup steps
      ...
      - name: Run drift detection
        id: drift
        run: |
          terramate run --cloud-sync-drift-status --cloud-sync-terraform-plan-file=drift.tfplan -- terraform plan -out drift.tfplan -detailed-exitcode
```
