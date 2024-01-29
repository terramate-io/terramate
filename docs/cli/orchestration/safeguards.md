---
title: Safeguards
description: Learn about the various safeguards provided by Terramate CLI that help you ensure that the code you're running is exactly what you want.
---

# Safeguards

Terramate CLI has a number of safeguards when executing commands with `terramate run`, to ensure that the code you're running against is *exactly what you intended*.

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
they *can* be disabled via CLI flags, environment variables or using the [project configuration](../projects/configuration.md).
Environment variables and CLI flags always take precedence over project config.

The `terramate.config.disable_safeguards` (added in v0.4.5) supports a list of check keywords to be disabled.
Below is a list of supported values:

| Check name | Description |
| --- | --- |
| all | Disable all checks |
| git | Disable all git related checks |
| git-untracked | Disable the check for untracked files |
| git-uncommitted | Disable the check for uncommitted files |
| git-out-of-sync | Disable the check for git remote out of sync |
| outdated-code | Disable the check for outdated code |

The keywords above can be used together in the environment variable `TM_DISABLE_SAFEGUARDS`,
in the `terramate.config.disable_safeguards` or provided in the command line in
the `--disable-safeguards=<options>`. Examples:

Using environment variable:
```
TM_DISABLE_SAFEGUARDS=git-untracked,git-uncommitted terramate run -- <cmd>
```

Using cli flag:

```
terramate run --disable-safeguards=git-untracked,git-uncommitted -- <cmd>
```

Using the configuration file:

```hcl
# terramate.tm
terramate {
  config {
    disable_safeguards = ["git-untracked", "git-uncommitted"]
  }
}
```

### Deprecated config and environment variables

The list of attributes and correspondent environment variable listed below are
deprecated (from v0.4.5 onwards) and will be removed in future versions of Terramate.

| Project configuration setting | Environment variable |
| --- | --- |
| `terramate.config.git.check_remote = false` | `TM_DISABLE_CHECK_GIT_REMOTE=true` |
| `terramate.config.git.check_untracked = false` | `TM_DISABLE_CHECK_GIT_UNTRACKED=true` |
| `terramate.config.git.check_uncommitted = false` | `TM_DISABLE_CHECK_GIT_UNCOMMITTED=true` |
| `terramate.config.run.check_gen_code = false` | `TM_DISABLE_CHECK_GEN_CODE=true` |

#### Example of a project config disabling all checks

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
