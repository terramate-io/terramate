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

This tutorial will demonstrate the basic concepts behind [Terramate](https://github.com/mineiros-io/terramate) and give you some ideas for how a filesystem-oriented code generator can help you to manage your Terraform code at scale more efficiently .
So that anyone can try Terramate without needing a cloud account, we will use only the `local_file` resource to create a static site demonstrating the basic principles behind Terramate. The only prerequisites are local installations of Terramate, Terraform and Git.

> We launched a [Terramate Discord Server](https://terramate.io/discord) in case you have any additional questions or issues running the examples in this guide.
>
> 👉 [https://terramate.io/discord](https://terramate.io/discord)

## Set up Terramate

> If you haven’t installed Terramate yet, please chose one of the various existing ways of how to install Terramate in ["Installation"](../installation.md).

Start by creating a new directory and create a file there called `terramate.tm.hcl` with the contents:

```hcl
# file: terramate.tm.hcl
terramate {
  config {
  }
}
```

Terramate uses the filesystem as its hierarchy and needs to know the project’s root, since all paths are relative to the project root, not the filesystem root.

In most circumstances, this would be the git root, but we’re explicitly marking the root with a blank config for now.

Next, `cd` into the new directory and run:

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

Any file with the extension `.tm` or `.tm.hcl` is recognized by Terramate and any Terramate file that contains a `stack {}` block is recognized as a stack.

A Terramate Stack is a directory that contains Terramate configuration files. This is where Terramate will generate Terraform. A stack has a 1:1 relationship with the Terraform state - i.e. each stack is a directory where you can run `terraform init`

Moreover, there is nothing special about this `stack.tm.hcl` file and it can be generated manually without the ID or the `terramate create` command.

However, it’s preferable to generate unique IDs for our stacks because, as we build more complex infrastructures, they allow us to reference stacks directly rather than using relative paths, which makes refactoring painful.

If we now run `terramate list` we should see our `mysite` directory as a listed stack. If you don't see it, you are probably running in the wrong directory; Terramate always uses the current working directory as the context for the command.

So, we have created a stack, but currently, it does nothing. We can run the command to generate our Terraform code, but it will do nothing, as no Terramate configuration is set up to generate code.

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

It should be clear from the code that this will generate an hcl file called `mysite.tf` which contains Terraform code for a `local_file` resource. Run `terramate generate` and it will create a `mysite.tf`file containing the content.

Creating native Terraform code is one feature that differentiates the Terramate approach from other solutions: No wrappers are necessary, just plain Terraform. If you cd into the directory and run `terraform init` and `terraform apply` it will do exactly what you expect: generate a local file in `/tmp/tfmysite/index.html` with the HTML code.

### Using Terramate Globals to generate dynamic code

Now, let’s make the content dynamic. Terramate uses variables called Globals. These can be defined in any Terramate file in any directory in a `globals {}` block. Each directory inherits all of the parent directory's globals and overwrites any with the same value. There are no complex precedence rules. The only rule is that sub-directories overwrite their parents if they declare a global of the same name. Simple!

So let’s define our title as a global in the root. Create a file in the root of the repository called `globals.tm.hcl` and put the following in:

```hcl
# file: globals.tm.hcl

globals {
  title = "My Website"
}
```

Note that there is nothing special about the name `globals.tm.hcl` and we could have put it in the existing `terramate.tm.hcl` because all Terramate files are merged at runtime (similar to how Terraform merges `.tf` files). This simplicity allows great flexibility to fit the way you choose to work.

Now in the `mysite/stack.tm.hcl` file, change the `<title>` to be `<title>${global.title}</title>`. If you now run `terramate generate` it should not change anything, but the title is now pulled from a global variable.

So, we have our working static site with a dynamic title, but now we’d like to split different versions into separate environments, development and production.

### Modularising the code

Let’s move our `mysite` stack into a "module" (where "module" here means "code that's not a stack and we’d like to reuse" - nothing to do with terraform modules).

In the project root, run:

```bash
mkdir -p modules/mysite
mv mysite/stack.tm.hcl modules/mysite/mysite.tm.hcl
rm -r mysite
```

In `modules/mysite/mysite.tm.hcl,` remove the `stack {}` block, since we don't want to generate code directly here, now it’s in the modules directory. We only want to import it elsewhere. It should now only have the `generate_hcl` block, and `terramate list` should now show no stacks because nowhere in the file tree is there a Terramate file with a `stack{}` block.

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

Let’s create production (prod) and development (dev) stacks. In the root dir:

```bash
terramate create dev/mysite
terramate create prod/mysite
```

You should now have a file structure that looks like this:

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

We want the `mysite` stack under the `dev` dir to be customized for a development environment and the same for prod. There are several ways to achieve this, but the simplest is to use more globals files situated in our environment subdirs. Since we can name them what we want, let's call them `dev/dev.tm.hcl` and `prod/prod.tm.hcl`:

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

So with this change, any stack we put under the prod directory will now inherit a `global.env == "prod"` (unless overwritten or explicitly unset), and the same for `dev`. Now we want our stack in each environment to import the `mysite` code, so in `<env>/mysite/stack.tm.hcl` we put the following:

```hcl
# file: <env>/mysite/stack.tm.hcl

import {
  source = "/modules/mysite/mysite.tm.hcl"
}
```

The only fix we need now is the output filename of the local_file (in the module at `/modules/mysite/stack.tm.hcl`), which is currently hard-coded, meaning dev and prod will overwrite each other. To solve this, we could use `${global.env}` in the path name, but let’s be fancy and use Terramate [metadata](../data-sharing/index.md#metadata)! In the `modules/mysite/mysite.tm.hcl` file, change the `local_file` resource `filename` attribute to use the metadata `${terramate.stack.path.relative}`:

```hcl
# in: modules/mysite/mysite.tm.hcl
generate_hcl "mysite.tf" {
  content {
    resource "local_file" "mysite" {
      filename = "/tmp/tfmysite/${terramate.stack.path.relative}/index.html"
...
```

To run our Terraform, we could now cd into `prod/mysite` and `dev/mysite` and run the necessary Terraform commands since it's just Terraform code, but doing things manually is tedious and prone to error as we scale, so it better would be to utilize Terramate: `terramate run` will run any ad-hoc command (such as `terraform init`) over the stacks we’ve created. In the project root directory, run `terramate run terraform init`:

```bash
$ terramate run terraform init
2023-04-06T14:32:47+01:00 ERR outdated code found action=checkOutdatedGeneratedCode() filename=dev/mysite/mysite.tf
2023-04-06T14:32:47+01:00 ERR outdated code found action=checkOutdatedGeneratedCode() filename=prod/mysite/mysite.tf
2023-04-06T14:32:47+01:00 FTL please run: 'terramate generate' to update generated code error="outdated generated code detected" action=checkOutdatedGeneratedCode()
```

Whoops. We should have run `terramate generate` before `terramate run`, but luckily Terramate knew that the generated code needed to be updated and it prevented us from running Terraform and performing actions we didn't want on outdated code.

So run

```bash
$ terramate generate
$ terramate run terraform init
```

again. You should now see Terraform initializing sequentially in `dev` and then `prod`.

`terramate run` executes in filesystem order by default, but [Terramate can control the order of execution](../orchestration/index.md#stacks-ordering) if needed.

Now run:

```bash
$ terramate run terraform apply
```

After approving, you should now have the rendered HTML files at `/tmp/tfmysite/<env>/mysite/index.html`

### Terramate Orchestration with Change Detection

To finish this quick introduction, let’s look at the globals precedence and one of Terramate’s most powerful features: change detection. Change detection is expected to be run in a CI-CD pipeline and requires a working git with a remote repository, so go to the root of your project and run

```bash
git init -b main
git add *
git commit -m 'initial commit'
```

and then set up a temporary local git remote to push to:

```bash
fake_github=$(mktemp -d)
git init -b main "${fake_github}" --bare
git remote add origin "${fake_github}"
git push --set-upstream origin main
```

In your project dir, git remote should now look something like this:

```bash
$ git remote -v
origin  /var/folders/dt/z_q3n2cs6r18364fhpbzzzmr0000gn/T/tmp.4ndijOue (fetch)
origin  /var/folders/dt/z_q3n2cs6r18364fhpbzzzmr0000gn/T/tmp.4ndijOue (push)
```

Let’s imagine we have a request to change the title for users when we’re looking at dev. Let’s follow the git workflow as if we were using GitHub, which will allow us to demonstrate change management. First, we create a new branch:

```bash
git checkout -b change-dev-title
```

Now, we want to overwrite the `mysite` title only in `dev`. Currently, `global.title` is set in the root `globals.tm.hcl`. As stated earlier, globals are inherited through the filesystem traversal so that we can overwrite `global.title` in any Terramate file in any parent of the stack (either `dev/` or `dev/mysite/`).

Let’s add it in `dev/dev.tm.hcl`:

```hcl
globals {
  env   = "dev"
  title = "THIS IS DEV"
}
```

As we build more complex stacks, seeing how the globals are evaluated in each stack can be helpful. To do this, run the (experimental — we’re still working on it) command `terramate experimental globals`:

```bash
$ terramate experimental globals
stack "/dev/mysite":
        env   = "dev"
        title = "THIS IS DEV"

stack "/prod/mysite":
        env   = "prod"
        title = "My Website"
```

Now run `terramate generate.` You should see that it modified the `dev/mysite/mysite.tf`:

```bash
$ terramate generate
Code generation report

Successes:

- /dev/mysite
        [~] mysite.tf

Hint: '+', '~' and '-' means the file was created, changed and deleted, respectively.
```

Commit the changed files with `git commit -am 'changed dev title'` and then run `terramate list --changed`. This shows you which stacks have outstanding changes from the main branch:

```bash
$ terramate list --changed
dev/mysite
```

If you now run `terramate run --changed terraform apply`, it will run the `apply` only against the changed stacks.

At Terramate.io, this change detection (via a GitHub Action) is a key part of our infrastructure workflows, allowing us to quickly and reliably build out large-scale infrastructure and we hope it can help you do the same.

### Conclusion

Hopefully, this tutorial helped you understand the fundamentals of Terramate and see where its simple code generation model using the filesystem hierarchy can help you organize your codebase and make it DRY without getting in your way.

Terramate is flexible, and there’s a lot more to [explore](https://terramate.io/docs/cli/), but we believe its power lies in its simplicity which allows you to integrate it easily, however you work, without having to invest in learning yet another API or complex tooling that further abstracts you from the native terraform code you’ve already built.

And if Terramate turns out not to work for you, no problem: just `rm` all the `*.tm{,.hcl}` files, and you're left with plain Terraform!

If you have questions or feature requests regarding Terramate, please join our [Discord Community](https://terramate.io/discord) or create an issue in the [Github repository.](https://github.com/mineiros-io/terramate)
