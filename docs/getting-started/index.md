---
title: Getting Started | Terramate
description: Terramate adds powerful capabilities such as code generation, stacks, orchestration, change detection, data sharing and more to Terraform.

prev:
  text: "Installation"
  link: "/installation"

next:
  text: "Stack Configuration"
  link: "/stacks/"
---

# Getting Started

[Terramate](https://github.com/mineiros-io/terramate) is a [code generator and orchestrator for Terraform](https://blog.terramate.io/product-introduction-github-as-code-af466550a4a9?source=friends_link&sk=ae60be77dcb484724b3b821898e7813d) that adds powerful capabilities such as code generation, stacks, orchestration, change detection, data sharing and more to Terraform.

This tutorial aims to introduce you to the basic concepts of [Terramate](https://github.com/terramate-io/terramate), and demonstrate how a filesystem-oriented code generator can improve the management of your Terraform code efficiently.

To follow this tutorial, you will need local installations of [Terramate](../installation.md), [Terraform](https://developer.hashicorp.com/terraform/tutorials/aws-get-started/install-cli), and [Git](https://git-scm.com/downloads).

Note that you won't need a cloud account, as we'll be using the `local_file` resource to create a static site that demonstrates Terramate's fundamental principles.

> We launched a [Terramate Discord Server](https://terramate.io/discord) in case you have any additional questions or issues running the examples in this guide.

> 👉 [https://terramate.io/discord](https://terramate.io/discord)

## Set up Terramate

> If you haven’t installed Terramate yet, please refer to the [installation](../installation.md) section for various options.

To start using Terramate, there are two possible ways of configuring a Terramate project. If you invoke Terramate inside
a Git repository, Terramate will automatically assume the top-level of your repository as the project root.
If you want to use Terramate in a directory that isn't a Git repository, you must configure the project root by creating
a `terramate.tm.hcl` configuration at the top level or any sub-directory in your Terramate project.

We will start working on a plain Terramate project without Git integration and add Git later to the project only.
To begin, create a new directory that contains a file named `terramate.tm.hcl` with the following content:

```hcl
# file: terramate.tm.hcl
terramate {
  config {
  }
}
```

Next, navigate `"cd"` to the new directory and run:

```bash
$ terramate create mysite
```

This creates a Terramate Stack, which is represented as a directory called `mysite` containing a file named `stack.tm.hcl` containing a `stack {}` block:

```hcl
# file: stack.tm.hcl

stack {
  name        = "mysite"
  description = "mysite"
  id          = "3b08c008-f0b2-4d01-8021-4c523199123e"
}
```

Terramate recognizes files with the `.tm` or `.tm.hcl` extension, and any file containing a `stack {}` block is considered a stack. A stack is simply a directory where Terramate generates Terraform files.

> **Note:** You can manually create the `stack.tm.hcl` file without an ID or by running the `terramate create` command.

> However, generating unique IDs for stacks is recommended since they enable direct stack referencing instead of relative paths, making refactoring difficult.

Run `terramate list` to see the `mysite/` directory listed as a stack. If it doesn't appear, ensure you're in the correct directory, as Terramate uses the current working directory as the context for executing all commands.

At this point, you have created a stack, but it does not perform any actions. Although you can generate Terraform code and run it without errors, it will not produce any results.

```bash
terramate generate
Nothing to do, generated code is up to date
```

## Terramate Code Generation

Let’s make it do something. Append the following to the `mysite/stack.tm.hcl`:

```hcl
# file: mysite/stack.tm.hcl
...
generate_hcl "mysite.tf" {
  content {
    resource "local_file" "mysite" {
      filename = "/tmp/tfmysite/index.html"
      content  = <<-EOF
        <html>
          <title>My Website</title>
        </html>
      EOF
    }
  }
}
```

The code provided demonstrates how to generate an HCL file named `mysite.tf` that contains Terraform code for a `local_file` resource. To create the `mysite.tf` file, run the `terramate generate` command. One of the key advantages of the Terramate is that it doesn't require any wrappers; **it seamlessly creates native Terraform code**.

Simply navigate to the directory, and execute `terraform init` and `terraform apply` to generate a local file in `/tmp/tfmysite/index.html` with the HTML code.

### Generate Dynamic Code with Terramate Globals

To **create dynamic content**, Terramate uses variables called [Globals](../data-sharing/index.md#globals). These variables can be defined in any Terramate file within a `globals {}` block. Each directory inherits all Globals from its parent directory, and any Globals with the same name will be overwritten. There are no complicated precedence rules. Subdirectories will overwrite parent directory Globals if they share the same name.

First, define a global variable called `title` in the root directory. Create a file named `globals.tm.hcl` and add the following content:

```hcl
# file: globals.tm.hcl

globals {
  title = "My Website"
}
```

The name `globals.tm.hcl` is not required, and the content could be included in an existing `terramate.tm.hcl` file. All Terramate files are merged during runtime, similar to how Terraform merges `.tf` files. This simplicity provides great flexibility.

Next, update the `mysite/stack.tm.hcl` file by replacing the `<title>` with `<title>${global.title}</title>`. Running `terramate generate` will not change anything, but the title now references a global variable.

With a dynamic title in place, you can create separate environments for development and production.

### Modularising the code

In this section, we will move our `mysite` stack into a "module" for reusability. A "module" here means "code that is not a stack and is intended for reuse" - unrelated to Terraform modules.

To do this, follow these steps:

1. Run the following commands in the project root:

```bash
mkdir -p modules/mysite
mv mysite/stack.tm.hcl modules/mysite/mysite.tm.hcl
rm -r mysite
```

2. In `modules/mysite/mysite.tm.hcl`, remove the `stack {}` block since code generation should no longer occur directly in the modules directory. This file should now only contain the `generate_hcl` block.

When running `terramate list`, no stacks should appear, as there are no Terramate files with a `stack{}` block within the file tree.

```bash
$ cat modules/mysite/mysite.tm.hcl
generate_hcl "mysite.tf" {
  content {
    resource "local_file" "mysite" {
      filename = "/tmp/tfmysite/index.html"
      content  = <<-EOF
        <html>
          <title>${global.title}</title>
        </html>
      EOF
    }
  }
}
```

Next, create production (prod) and development (dev) stacks in the root directory:

```bash
terramate create dev/mysite
terramate create prod/mysite
```

Your file structure should now look like this:

```bash
dev/
  mysite/
    stack.tm.hcl
modules/
  mysite/
    mysite.tm.hcl
prod/
  mysite/
    stack.tm.hcl
globals.tm.hcl
terramate.tm.hcl
```

We want to customize `mysite` stack under the `dev` directory for a development environment and the same for prod. To achieve this, use additional globals files located in our environment subdirectories, named `dev/dev.tm.hcl` and `prod/prod.tm.hcl`:

```hcl
# file: dev/dev.tm.hcl

globals {
  env = "dev"
}
```

```hcl
# file: prod/prod.tm.hcl

globals {
  env = "prod"
}
```

With this change, any stack under the prod directory will inherit a `global.env == "prod"` (unless overwritten or explicitly unset), and the same applies to `dev`. Now we want to import `mysite` code into the stack in each environment. In `<env>/mysite/stack.tm.hcl` insert the following:

```hcl
# file: <env>/mysite/stack.tm.hcl

import {
  source = "/modules/mysite/mysite.tm.hcl"
}
```

Update the output filename of the local_file resource in `/modules/mysite/stack.tm.hcl` to avoid overwriting between dev and prod. Use the Terramate metadata `${terramate.stack.path.relative}` in the path name:

```hcl
# in: modules/mysite/mysite.tm.hcl
generate_hcl "mysite.tf" {
  content {
    resource "local_file" "mysite" {
      filename = "/tmp/tfmysite/${terramate.stack.path.relative}/index.html"
...
```

To execute Terraform, navigate `"cd"` to `prod/mysite` and `dev/mysite` and run the necessary Terraform commands.

However, to avoid manual execution and potential errors, run `terramate run terraform init` in the project root directory.

```bash
$ terramate run terraform init
2023-04-06T14:32:47+01:00 ERR outdated code found action=checkOutdatedGeneratedCode() filename=dev/mysite/mysite.tf
2023-04-06T14:32:47+01:00 ERR outdated code found action=checkOutdatedGeneratedCode() filename=prod/mysite/mysite.tf
2023-04-06T14:32:47+01:00 FTL please run: 'terramate generate' to update generated code error="outdated generated code detected" action=checkOutdatedGeneratedCode()
```

Whoops, we have errors!

We should have run `terramate generate` before `terramate run`. Thankfully, Terramate detected that the generated code needed to be updated and prevented us from running Terraform with outdated code.

To fix this, run `terramate generate`, followed by `terramate run terraform init` again. Terraform should now initialize sequentially in dev and prod. By default, terramate run executes in filesystem order, but [Terramate can control the order of execution](../orchestration/index.md#stacks-ordering) if needed.

Now run:

```bash
$ terramate run terraform apply
```

After approval, the rendered HTML files should be located at `/tmp/tfmysite/<env>/mysite/index.html`

### Terramate Orchestration with Change Detection

To conclude this introduction, let's explore globals precedence and one of Terramate's most powerful features: change detection. [Change detection](../change-detection/index.md) is designed for CI-CD pipelines and requires a working Git repository with a remote. In your project's root, run:

```bash
git init -b main
git add *
git commit -m 'initial commit'
```

Next, set up a temporary local git remote to push to:

```bash
fake_github=$(mktemp -d)
git init -b main "${fake_github}" --bare
git remote add origin "${fake_github}"
git push --set-upstream origin main
```

In your project directory, `git remote -v` should now look something like this:

```bash
$ git remote -v
origin  /var/folders/dt/z_q3n2cs6r18364fhpbzzzmr0000gn/T/tmp.4ndijOue (fetch)
origin  /var/folders/dt/z_q3n2cs6r18364fhpbzzzmr0000gn/T/tmp.4ndijOue (push)
```

Imagine we need to change the title for users in the `dev` environment. To demonstrate change management, let's create a new branch

```bash
git checkout -b change-dev-title
```

We now want to overwrite the `mysite` title only in `dev`. Currently, `global.title` is set in the root `globals.tm.hcl`. Globals are inherited through the filesystem traversal, so we can overwrite `global.title` in any Terramate file in any parent of the stack (either `dev/` or `dev/mysite/`). Add it to `dev/dev.tm.hcl`:

```hcl
globals {
  env   = "dev"
  title = "THIS IS DEV"
}
```

To view how globals are evaluated in each stack, run the experimental command `terramate experimental globals`:

```bash
$ terramate experimental globals
stack "/dev/mysite":
        env   = "dev"
        title = "THIS IS DEV"

stack "/prod/mysite":
        env   = "prod"
        title = "My Website"
```

Run `terramate generate.` You should see that it modified `dev/mysite/mysite.tf`:

```bash
$ terramate generate
Code generation report

Successes:

- /dev/mysite
        [~] mysite.tf

Hint: '+', '~' and '-' means the file was created, changed and deleted, respectively.
```

Commit the changed files with `git commit -am 'changed dev title'`, then run `terramate list --changed`. This command displays stacks with outstanding changes compared to the main branch:

```bash
$ terramate list --changed
dev/mysite
```

Now, if you execute `terramate run --changed terraform apply`, it will apply the changes only to the affected stacks.

### Conclusion

We hope this tutorial has helped you grasp the basics of Terramate, and demonstrated how its simple code generation model works based on the filesystem hierarchy, and how it can assist you in organizing your codebase and maintaining its DRYness without any complications.

Terramate is designed to be flexible, and there is a wealth of features to explore. Its power lies in its simplicity, enabling seamless integration with your workflow without requiring you to invest time in learning another API, or complex tooling that further distances you from the native Terraform code you've already built.

If Terramate doesn't meet your current needs, no worries: simply remove all the `*.tm{,.hcl}` files, and you're back to using plain Terraform! If you have questions or feature requests regarding Terramate, we encourage you to join our [Discord Community](https://terramate.io/discord) or create an issue in the [Github repository.](https://github.com/terramate-io/terramate)
