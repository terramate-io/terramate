---
title: Safeguards
description: Learn about the various safeguards provided by Terramate CLI that help you ensure that the code you're running is exactly what you want.
---

# Safeguards

Terramate CLI has a number of safeguards when executing commands with `terramate run`, to ensure that the code you're running
against is *exactly what you intended*.

## Git checks

The first set of checks ensures that your git status is clean:

- No files exist that are untracked (`check_untracked`)
- No changes exist that are uncommitted (`check_uncommited`)
- Local git is up to date with remote changes (`check_remote`)

## Code generation check

By default, `terramate run` will check that all generated code is up to date and throw an error if not:

```sh
ERR outdated code found action=checkOutdatedGeneratedCode() filename=stg/ec2/_provider.tf
FTL please run: 'terramate generate' to update generated code error="outdated generated code detected" action=checkOutdatedGeneratedCode()

```

This check ensures that it's not possible to accidentally run against outdated code and we *highly* discourage disabling it.

We also recommend using git hooks (e.g. [pre-commit](https://pre-commit.com/)) to ensure that `terramate generate` is run and the code is up to date *before* pushing your changes.

## Disabling checks

Safeguards are enabled per default and help you to keep your environment safe, but if required
they *can* be disabled either via environment variable or via the [project configuration](../projects/configuration.md).
Environment variables always take precedence over project config.

| Project configuration setting | Environment variable |
| --- | --- |
| `terramate.config.git.check_remote = false` | `TM_DISABLE_CHECK_GIT_REMOTE=true` |
| `terramate.config.git.check_untracked = false` | `TM_DISABLE_CHECK_GIT_UNTRACKED=true` |
| `terramate.config.git.check_uncommitted = false` | `TM_DISABLE_CHECK_GIT_UNCOMMITTED=true` |
| `terramate.config.run.check_gen_code = false` | `TM_DISABLE_CHECK_GEN_CODE=true` |

### Example of a project config disabling all checks

```hcl
# terramate.tm.hcl
terramate {
  config {
    run {
      check_gen_code = false
    }
    git {
      check_remote      = false
      check_untracked   = false
      check_uncommitted = false
    }
  }
}

```
