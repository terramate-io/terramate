---
title: Drift Notifications
description: Learn how to configure Slack notifications for drift detection in Terramate Cloud.
---

# Drift Notifications

When the Slack WebHook URL is configured in the [General Organization settings](../organization/settings.md), notifications will be sent to the corresponding Slack Channel.

## New Drift Notification

A notification will be sent for every stack that newly drifts even
when multiple stacks drift in the same drift check run.

When a stack is already in a drifted state, no notification will be sent anymore until the drift is resolved and the stack newly drifts again.

The notification will contain detailed information about the drifted stack and link to Terramate Cloud to see also the detailed drift as a Terraform Plan.

## Initial Drift Run

When onboarding to Terramate Cloud it is recommended to run a drift run before setting up the notifications when the initial run is expected to detect a lot of drifted stacks.

After the initial drift check run, the Slack Webhook can be set up and only new drifts will notify the channel.
