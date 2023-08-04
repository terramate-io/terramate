---
title: Project Configuration
description: Learn how to configure a Terramate project. Terramate does not depend on user configuration and comes with a set of sensible defaults.

prev:
  text: 'Projet Setup'
  link: '/configuration/project-setup'

next:
  text: 'Upgrade Check'
  link: '/configuration/upgrade-check'
---

# Project Configuration

Terramate does not depend on user configuration and comes with a set of sensible defaults.

Configurations are valid project-wide and can be defined only once at the project root.

All project configurations are defined within the `terramate` block.

## The `terramate.required_version` attribute

Required version is defined by the attribute `terramate.required_version` attribute
where `required_version` accepts a version constraint string that specifies which
versions of Terramate can be used inside a Terramate project.

The `terramate.required_version_allow_prereleases` attribute controls if **prereleases** should be matched according to
[Semantic Versioning](https://semver.org/) precedence ordering. It's **false** by
default and if set to `true`, then Terramate will also accept prereleases if they
match the provided constraint.

>It is generally advised against modifying infrastructure code without possessing a comprehensive understanding of its function and impact. Ensure extreme caution when considering experimental releases, given that they can potentially lead to dangerous changes.
>

We recommend pinning the exact version to use and updating the config when updating Terramate.

The version constraint string uses the same notation as the one used on
[Terraform](https://www.terraform.io/language/expressions/version-constraints).

Valid examples:

```hcl
terramate {
  # allow any Terramate v0.1.x version starting at v0.1.8
  required_version = "~> 0.1.8"
}
```

```hcl
terramate {
  # allow any Terramate v0.1.x version starting at v0.1.8
  required_version = ">= 0.1.8, < 0.2.0"
}
```

If a `terramate.required_version` is not defined within a project, no versioning checks will be carried
out. If a version constraint is specified and an incompatible version of Terramate is run, a fatal error
will occur.

## The `terramate.config` block

Project-wide configuration can be defined in this block. All possible settings are described in the following subsections.

### The `terramate.config.git` block

Git related configurations are defined inside the `terramate.config.git` block, like this:

```hcl
terramate {
  config {
    git {
      default_remote = "origin"
      default_branch = "main"
      check_untracked = false
      check_uncommitted = false
      check_remote = false
    }
  }
}
```

For a comprehensive list of configurations and their corresponding schema, refer to the
[configuration overview](index.md).

### The `terramate.config.run` Block

Configuration for the `terramate run` command can be set in the
`terramate.config.run` block.

#### The `terramate.config.run.env` Block

In `terramate.config.run.env` block a map of environment variables can be defined
that will be set when running a command using `terramate run`.

The following example exports the `TF_PLUGIN_CACHE_DIR` environment variable to
enable [Terraform Provider Plugin Caching](https://www.terraform.io/cli/config/config-file#provider-plugin-cache).

```hcl
terramate {
  config {
    run {
      env {
        TF_PLUGIN_CACHE_DIR = "/some/path/etc"
      }
    }
  }
}
```

Inside the `terramate.config.run.env` block the `env` namespace will be
available including environment variables retrieved from the host:

```hcl
terramate {
  config {
    run {
      env {
        TF_PLUGIN_CACHE_DIR = "${env.HOME}/.terraform-cache-dir"
      }
    }
  }
}
```

In addition Globals (`global.*`) and Metadata (`terramate.*`) are available and
are evaluated lazy withing the stack context the commands are executed in.

The `env` namespace is meant to give access to the host environment variables,
it is read-only, and is only available when evaluating
`terramate.config.run.env` blocks.

Any attributes defined
on `terramate.config.run.env` blocks won't affect the `env` namespace.

You can have multiple `terramate.config.run.env` blocks defined on different
files, but variable names **cannot** be defined twice.
