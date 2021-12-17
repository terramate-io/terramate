# Project Config

Terramate does not depend on user configuration and comes with a set of sensible defaults.
Configurations allow you to override any terramate default value.

There can be only one Terramate project configuration on the entire project and
it **must** be located at the project root dir.

Project wide configuration is done via a **config** block inside the
**terramate** block:

```
terramate {
  config {
    // project wide configurations here
  }
}
```

## Git

Git related configurations are defined inside the **git** block, like this:

```
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
