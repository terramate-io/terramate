---
title: Runtime Configuration
description: Learn how to configure environment variables available to the terramate run command.
---

# Runtime Configuration

The `terramate.config.run.env` block can be used to define a map of environment variables
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
are evaluated lazily within the stack context that commands are executed in.

The `env` namespace is meant to give access to the host environment variables,
it is read-only and is only available when evaluating `terramate.config.run.env` blocks.

Any attributes defined in `terramate.config.run.env` blocks won't affect the `env` namespace.

You can have multiple `terramate.config.run.env` blocks defined in different
files, but variable names **cannot** be defined twice.
