# Project Configuration

Terramate does not depend on user configuration and comes with a set of sensible defaults.

Configurations are valid project-wide and can be defined only once at the project root.

All project configurations are defined within the `terramate` block.

## The `terramate.required_version` Attribute

Required version is defined by the attribute `terramate.required_version` attribute
where `required_version` accepts a version constraint string,
which specifies which versions of Terramate can be used inside a Terramate project.

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

No versioning check will be performed if no `terramate.required_version` is defined on a project. If a version constraint is defined, running `terramate` with an incompatible version will result in a fatal error.

## The `terramate.config` Block

Project-wide configuration can be defined in this block. All possible settings are described in the following subsections.

### The `terramate.config.git` Block

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

For a list of all configurations and their full schema check the
[configuration overview](config-overview.md#terramateconfiggit-block-schema).

### The `terramate.config.run` Block

Configuration for the `terramate run` command can be set in the
`terramate.config.run` block.

For a list of all configurations and their full schema check the
[configuration overview](config-overview.md#terramateconfigrun-block-schema).

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
files, but variable names can **not** be defined twice.
