---
title: Upgrade and Security Bulletin Checks
description: The Terramate CLI commands interacts with the Terramate Checkpoint service to check for the availability of new versions and for critical security bulletins about the current version.
---

# Upgrade and Security Bulletin Checks

The Terramate CLI commands interacts with the [Terramate Checkpoint](https://checkpoint-api.terramate.io/)
service to check for the availability of new versions and for critical security
bulletins about the current version.

The Checkpoint service is based on Hashicorp's [Checkpoint](https://checkpoint.hashicorp.com/)
which is used by most of theirs products, including Terraform.

One place where the effect of this can be seen is in `terramate version`, where it
is used by default to indicate in the output when a newer version is available.

Only _anonymous information_, which cannot be used to identify the user or host, is
sent to Checkpoint. An anonymous ID is sent which helps de-duplicate warning
messages. Both the anonymous id and the use of checkpoint itself are completely
optional and can be disabled.

Checkpoint itself can be entirely disabled by setting the environment variable
`CHECKPOINT_DISABLE` to any non-empty value.

The [Checkpoint client code](https://github.com/terramate-io/go-checkpoint) used
by Terramate is available for review by any interested party
