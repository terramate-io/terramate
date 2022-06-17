# Project Configuration

Terramate does not depend on user configuration and comes with a set of sensible defaults.
But when defaults doesn't work Terramate allows some project wide configurations
to be changed.

Configurations that are project wide can be defined only once, and they **MUST**
be defined at the project root.

## terramate.required_version

Required version is defined by the attribute `terramate.required_version`
where `required_version` accepts a version constraint string,
which specifies which versions of Terramate can be inside a Terramate project.

The version constraint string uses the same notation as the one used on
[Terraform](https://www.terraform.io/language/expressions/version-constraints).

Valid examples:

```hcl
terramate {
    required_version = "~> 0.0.11"
}
```

```hcl
terramate {
    required_version = ">= 1.2.0, < 2.0.0"
}
```

If no `terramate.required_version` is defined on a project, no versioning
check will be performed. If one is defined, running `terramate` with a
incompatible version will result in an error for any Terramate command.

## terramate.config.git

Git related configurations are defined inside the **git** block, like this:

```hcl
terramate {
  config {
    git {
      default_remote = "origin"
      default_branch = "main"
    }
  }
}
```

List of configurations:

* git.default\_remote (default="origin") : changes default remote
* git.default\_branch (default="main")   : changes default branch


## terramate.config.run.env

The environment in which stacks are going to be executed can be configured
using the `terramate.config.run.env` block. This block has no labels and
accepts arbitrary attributes where each attribute represents an environment
variable that will be evaluated and exported when executing the stack.
The attributes must always evaluate to strings.

Example:

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

Will export the environment variable `TF_PLUGIN_CACHE_DIR` on the stack
execution environment with the value `/some/path/etc`.

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

Globals and Metadata can also be referenced:

```hcl
terramate {
  config {
    run {
      env {
        TF_PLUGIN_CACHE_DIR = "${terramate.stack.path.absolute}/${global.cache_dir}"
      }
    }
  }
}
```

The `env` namespace is meant to give access to the host environment variables,
it is read-only, and is only available when evaluating
`terramate.config.run.env` blocks.

This means that any attributes defined
on `terramate.config.run.env` blocks won't affect the `env` namespace,
they only affect the stack execution environment.

Given that this is a project wide configuration the environment defined by
`terramate.config.run.env` will be available to all stacks in the project.

You can have multiple `terramate.config.run.env` blocks defined on different
files, but no attribute must be redefined on any of them.

This is allowed:

```hcl
terramate {
  config {
    run {
      env {
        A = "A"
      }
    }
  }
}

terramate {
  config {
    run {
      env {
        B = "B"
      }
    }
  }
}
```

But this will fail:

```hcl
terramate {
  config {
    run {
      env {
        A = "A"
      }
    }
  }
}

terramate {
  config {
    run {
      env {
        A = "A"
      }
    }
  }
}
```
