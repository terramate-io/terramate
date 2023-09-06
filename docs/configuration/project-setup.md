---
title: Create a new Terramate project
description: Learn how to set up a new Terramate project. Terramate comes with sensible defaults so just using it inside a pre existent Git repository should not require any configurations.

prev:
  text: 'Terramate Configuration'
  link: '/configuration/'

next:
  text: 'Project Configuration'
  link: '/configuration/project-config'
---

# Create a new Terramate project

If you already have a project versioned on Git setting up
Terramate is as easy as just [installing Terramate](./../installation.md).
Terramate comes with sensible defaults so just using it inside a pre existent
Git repository should not require any configurations.

The exception being repositories that have a default remote branch
other than `origin/main`, in that case to make change detection work you will
need to set a customized [project configuration](project-config.md).

If you want to play around with Terramate from scratch locally you can also
setup a local git repository:

```sh
playground=$(mktemp -d)
local_origin=$(mktemp -d)

git init -b main "${playground}"
git init -b main "${local_origin}"  --bare

cd "${playground}"
git remote add origin "${local_origin}"

echo "My Terramate Playground" > README.md

git add README.md
git commit -m "first commit"
git push --set-upstream origin main

# Start using terramate
```

Terramate can also work without any VCS setup, it will only require
a Terramate configuration at the top level directory of the project

```sh
playground=$(mktemp -d)
cd "${playground}"

cat > terramate.tm.hcl <<- EOM
terramate {
  config {
  }
}
EOM

# Start using terramate
```

In a setup with no VCS, [change detection](../change-detection/index.md) features will not be available.

You can also check our [live example](https://github.com/terramate-io/terramate-example-code-generation).

# Initializing existing infrastructure

If you already have a IaC repository containing multiple Terraform stacks then
you can use the command below to easily detect and create Terramate stacks automatically:

```bash
terramate create --all-terraform
```

The command above will scan the repository looking for Terraform directories
containing a `backend` or `provider` configuration, which commonly indicates that
it's a Terraform stack.
