---
title: Create an Account for Terramate Cloud
description: Learn how to sign up for a free account for Terramate Cloud, invite your team members, connect your VCS, configure Slack alerts and authenticate Terramate CLI.
---

# Terramate Cloud Getting Started

## Sign up as a new user

### First Time Sign in

When signing up to the platform at [cloud.terramate.io](https://cloud.terramate.io/), you are asked to choose a social
login provider to sign in with.

Terramate Cloud offers to sign in using:

- A Google Workspace Account (formerly known as GSuite Account),
- A GitHub Account
- A Microsoft Entra ID Account

### Configuring your Profile

Upon the first sign-in, your profile will need to be created. You need to choose a display name and set your company position. Using your real name or a name your team recognizes is recommended.

### Creating or Joining an Organization

After setting up your profile, you can join an organization you are invited to or create a new one.

You can be a member of multiple organizations simultaneously and thus part of multiple teams.

Select the “join” button to join an organization, and you will become an active member. After this, you can “visit” your newly joined organization.

Creating a new organization is as easy as joining one.

You can choose a display name of your new organization and an organization's short name. The short name will be used in URLs (`https://cloud.terramate.io/o/{short-name}`) when visiting the organization or in Terramate CLI when selecting the organization to sync or receive data from.

### Inviting your team

After creating a new organization, you can invite your teammates by e-mail.

You can select any number of e-mail addresses to invite, or you can skip this step to invite your team later from the Organization Management area.

### Configure Slack alerts

In the general settings of your organization you can configure your Slack webhook to receive notifications related to
deployments, drift detection, pull requests, and more.

### The dashboard

Initially, you will be located on the organization's dashboard. If no data has been synchronized to your organization so far, instructions to do so will lead you to this documentation.

## Connecting the CLI

You can use [`terramate cloud login`](../../cli/cmdline/cloud/cloud-login.md) to log in to Terramate Cloud. A browser window
will allow you to select a sign-in method of your choice.

You need to select the same account you just signed up with to use Terramate CLI with your Terramate Cloud Organization.

You can validate you are connected to the correct Terramate Cloud Organization using [`terramate cloud info`](../../cli/cmdline/cloud/cloud-info.md):

::: code-group

```sh [shell]
terramate cloud info
```

```sh [output]
status: signed in
provider: Google Social Provider
user: Your Display Name
email: you@example.com
organizations: example
```

:::

After successful sign-in via Terramate CLI, it is recommended to persist the selected cloud organization to your configuration
by creating a config section in e.g., your `terramate.tm.hcl` file as shown here, but replacing `"example"` with the selected
short name of your organization:

```bash
terramate {
  config {
    cloud {
      organization = "example"
    }
  }
}
```

Terramate CLI is now aware of Terramate Cloud and can be used to synchronize data. For example, to retrieve a list of
all stacks that are drifted in Terramate Cloud you can run `terramate list --cloud-status=drifted`.
