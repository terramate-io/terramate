---
title: terramate cloud drift show - Command
description: With the terramate cloud drift show command you can view the drift that has occurred on a stack

prev:
  text: 'Clone'
  link: '/cli/cmdline/clone'

next:
  text: 'Cloud Info'
  link: '/cli/cmdline/cloud-info'
---

# Cloud Drift Show

**Note:** This is an experimental command that is likely subject to change in the future.

The `cloud drift show` command shows any drift that has occurred in the current stack in the working directory (or use `-C` to point to the directory). You must be [logged into](./cloud-login.md) to your Terramate Cloud account.

## Usage

`terramate -C <stack-directory> experimental cloud drift show`
