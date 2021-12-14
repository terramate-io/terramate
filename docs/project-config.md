# Project Config

Terramate does not require any configurations to work, but in case default
behavior is undesirable, these configurations allow you to override it
on a per project basis.

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
