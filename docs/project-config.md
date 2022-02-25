# Project Config

Terramate does not depend on user configuration and comes with a set of sensible defaults.
But when defaults doesn't work Terramate allows some project wide configurations
to be changed.

Configurations that are project wide can be defined only once, and they **MUST**
be defined at the project root.

Here it is documented all Terramate configurations that are project wide.

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
