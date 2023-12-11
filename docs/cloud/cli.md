---
title: Setting up Terramate CLI for the Cloud
description: Terramate CLI has functionality that integrates with Terramate Cloud
---

# Setting up Terramate CLI for the Cloud

## Terramate Cloud functionality in the local CLI

Users of Terramate Cloud have additional functionality available to them in Terramate CLI. In order to use this functionality locally you need to run `terramate experimental cloud login`. This will open cloud.terramate.io in your browser and attempt to sign you in.

You can confirm you are successfully logged in by running `terramate experimental cloud info`. This will also tell you what organizations you belong to. If you belong to multiple multiple organizations, make sure that any local commands you run are targetting the correct organization by either:

- Setting the `terramate.config.cloud.organization` option in the project [configuration file](/docs/cli/configuration/project-config.md) (the `terramate.tm.hcl` file at the root of your repository)
- Setting the `TM_CLOUD_ORGANIZATION` environment variable

### Local CLI functionality

`terramate list --experimental-status=<status>` will list all stacks in the repo where you run the command that have a Cloud status of one of the following:

| Status      | Meaning                                                          |
| ----------- | ---------------------------------------------------------------- |
| `ok`        | The stack is not drifted and the last deployment succeeded       |
| `failed`    | The last deployment of the stack failed so the status is unknown |
| `drifted`   | The actual state is different from that defined in code          |
| `unhealthy` | The stack is drifted or failed                                   |
| `healthy`   | The stack is ok                                                  |

`terramate experimental trigger --experimental-status=<status>` will create a trigger for all stacks with the corresponding status

## Terramate CLI for the Cloud in CI/CD

In order to syncronize deployment data to Terramate Cloud and perform drift detection you will need to adjust your CI/CD pipelines.

When running from Github Actions authentication is not necessary if you have connected your Terramate Cloud account to your Github Org, but in order for Terramate Cloud to sync Github metadata your Action will need GITHUB_TOKEN set as an environment variable and the appropriate permissions, e.g.

```
jobs:
  deploy:
    name: Deploy

    permissions:
      id-token: write # necessary for GITHUB_TOKEN to work

    env:
      GITHUB_TOKEN: ${{ github.token }}

    ...
```

### Syncing Deployments

In your Github Action workflow you will need to adjust the `terramate run` commands to sync the deployment to the cloud using the `--cloud-sync-deployment` argument e.g.:

```
jobs:
  deploy:
    name: Deploy
    ...
      - name: Apply changes
        id: apply
        run: terramate run --changed --cloud-sync-deployment -- terraform apply -input=false -auto-approve
```

### Detecting Drift

To detect drift, you will need to create a [scheduled action](https://docs.github.com/en/actions/using-workflows/events-that-trigger-workflows#schedule). The `terramate run` command supports `--cloud-sync-drift-status` which will set any stack to drifted _if the exit code of the command that's run is `2`_ (which for `terraform plan -detailed-exitcode` signals that the plan succeeded and there was a diff). Terramate is also able to sync the drifted plan with the `--cloud-sync-terraform-plan-file` option, so a typical action for drift detection would look something like:

```
name: Check drift on all stacks once a day

on:
  schedule:
    # * is a special character in YAML so you have to quote this string
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
      - name: Run drift detection
        id: drift
        run: |
          terramate run --cloud-sync-drift-status --cloud-sync-terraform-plan-file=drift.tfplan -- terraform plan -out drift.tfplan -detailed-exitcode
```
