---
title: Project Configuration
description: Learn how to configure a Terramate project. Terramate does not depend on user configuration and comes with a set of sensible defaults.
---

# Project Configuration

Any Git repository is considered to be a Terramate project. Additionally, Terramate does not depend on user configuration
and comes with a set of sensible defaults that can be overwritten by adding a `terramate.tm.hcl` file at the
root of your repository.

Configurations are valid project-wide and can be defined only once at the project root.
All project configurations are defined within the `terramate` block.

```hcl
# terramate.tm.hcl

terramate {
  # allow any Terramate v0.4.x version starting at v0.4.3
  required_version = "~> 0.4.3"

  config {
    # config options
  }
}

```


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
  # allow any Terramate v0.4.x version starting at v0.4.3
  required_version = "~> 0.4.3"
}
```

```hcl
terramate {
  # allow any Terramate v0.4.x version starting at v0.4.3
  required_version = ">= 0.4.3, < 0.5.0"
}
```

If a `terramate.required_version` is not defined within a project, no versioning checks will be carried
out. If a version constraint is specified and an incompatible version of Terramate is run, a fatal error
will occur.

## The `terramate.config` block

Project-wide configuration can be defined in this block. All possible settings are described in the following subsections.

### The `terramate.config.git` block

[Git integration](../change-detection/integrations/git.md) related configurations used in the
[Change Detection](../change-detection/index.md) and [Safe Guards](../orchestration/safeguards.md)
are defined inside the `terramate.config.git` block, like this:

```hcl
terramate {
  config {
    git {
      # Git configuration
      default_remote = "origin"
      default_branch = "main"

      # Safeguard
      check_untracked   = false # Deprecated as of v0.4.5 (use terramate.config.disable_safeguards instead)
      check_uncommitted = false # Deprecated as of v0.4.5 (use terramate.config.disable_safeguards instead)
      check_remote      = false # Deprecated as of v0.4.5 (use terramate.config.disable_safeguards instead)
    }
  }
}
```

### The `terramate.config.generate` block

The `terramate.config.generate` block can be used to configure the code generate feature.
For now, only the `hcl_magic_header_comment_style` attribute is supported and it can be used to
define which HCL comment style must be used by Terramate when generating HCL files.
Example below:

```
terramate {
  config {
    generate {
      hcl_magic_header_comment_style = "#"
    }
  }
}
```

The config above will make Terramate generate files using `#` as comment style.
The only valid options are `//` and `#`.

### The `terramate.config.run` Block

Configuration for the `terramate run` command can be set in the `terramate.config.run` block.

#### Disable code generation check

By default, `terramate run` will check that all generated code is up to date and throw an error if not:

```sh
ERR outdated code found action=checkOutdatedGeneratedCode() filename=stg/ec2/_provider.tf
FTL please run: 'terramate generate' to update generated code error="outdated generated code detected" action=checkOutdatedGeneratedCode()
```

If you want to disable this default safe-guard, you can set `check_gen_code = false` in the `terramate.config.run` block.

```hcl
terramate {
  config {
    run {
      check_gen_code = false # Deprecated as of v0.4.5 (use terramate.config.disable_safeguards instead)
    }
  }
}
```

::: tip
This check ensures that it's not possible to accidentally run against outdated code and we discourage disabling it.
:::

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

In addition, Globals (`global.*`) and Metadata (`terramate.*`) are available and
are evaluated lazy within the stack context the commands are executed in.

The `env` namespace is meant to give access to the host environment variables,
it is read-only, and is only available when evaluating
`terramate.config.run.env` blocks.

Any attributes defined
on `terramate.config.run.env` blocks won't affect the `env` namespace.

You can have multiple `terramate.config.run.env` blocks defined on different
files, but variable names **cannot** be defined twice.

### The `terramate.config.cloud` block

Properties related to Terramate Cloud can be defined inside the `terramate.config.cloud` block.
Currently, this block is only used to set the default cloud organization name:
```hcl
terramate {
  config {
    cloud {
      organization = "my-org-name"
    }
  }
}
```
Setting a cloud organization name is required when
* syncing with Terramate Cloud, i.e. by using `terramate run` with the `--cloud-sync-drift-status` or `--cloud-sync-deployment` options, and
* the user is a member of more than one cloud organization.

The specified name will be used to select which of the user's organizations to use in the scope of the project.

It's also possible to select a cloud organization by setting the environment variable `TM_CLOUD_ORGANIZATION` to the organization name. If set, the value from the environment variable will override the configuration setting.
