---
title: Deployment Notifications
description: Learn how to get notified about new deployments via Terramate Cloud.
---

# Deployment Notifications

When the Slack WebHook URL is configured in the [General Organization settings](../organization/settings.md), notifications will be sent to the corresponding Slack Channel.

## Deployment Notifications

For every deployment, a notification will be sent that summarizes the results of all stacks involved in the deployment.

The number of successful and failed deployments will be shown and links to deployment details pages in Terramate Cloud will grant access to the logs of the failed deployments and show each stack's detailed status.

## Deployment Grouping

When a deployment is run via multiple workflows or within a matric (e.g. in GitHub Actions) Terramate Cloud will group the different flows with a best effort strategy. In rare cases, this can lead to postponed grouping and duplicated notifications.

When Deployment grouping does not succeed, multiple deployments will be shown in the Terramate cloud.
