# Getting Started

Welcome to Terramate documentation! This page will give you an introduction to
the 80% of Terramate concepts that you will use on a daily basis.

> You will learn:
> 
> - Setup the project
> - How to create stacks
> - Manage globals
> - Generate Terraform code
> - Generate custom files
> - Orchestrate your stack's execution
> - How change detector works

No cloud account will be needed for this tutorial as we will play with pure
Terramate.

## Project Setup

If you don't have Terramate installed, then first head to the 
[installation](./installation.md) page and follow the steps there.

If you are new to Terramate or if you are creating a new project using it,
make sure you have the latest version installed. The command `terramate version` 
will inform you if your installed version is not the latest.

```shell
$ terramate version
0.2.16

Your version of Terramate is out of date! The latest version
is 0.2.17 (released on Mon Apr 3 00:00:00 UTC 2023).
You can update by downloading from https://github.com/mineiros-io/terramate/releases/tag/v0.2.17
```

Terramate has some features for enhancing the [git](https://git-scm.com/) workflow
of infrastructe as code (IaC) projects, then because of that it's better if
you start setting up a complete git repository, otherwise some features explained
here won't work.

### Setting up the repository

Terramate comes with sensible defaults for repositories created in 
[GitHub](https://github.com), [Gitlab](https://gitlab.com) or [Bitbucket](https://bitbucket.com) version control hostings, then the easiest way for getting started
is just create a repository in any of them, and clone the repository on your
machine.

If your company has a dedicated git server, there's a good chance that everything
will work smoothly as well, the exception being repositories that have a default
remote branch different than `origin/main`, and if that's the case you will need
additional configuration explained in the [project configuration](project-config.md)
page.

It's important that you start with a *cloned* repository instead of a locally
initialized git repository because we need a fully functioning repository, ie
default branch must have an initial commit, remote/upstreams must be set and
working, etc.

Let's say you cloned the repository into a `my-iac` directory:

```shell
$ git clone <url> my-iac
```

Then if you `cd` into that directory and execute Terramate commands, it will
detect it as a valid _Terramate Project_. In other words, any git project is a
valid _Terramate Project_. The Terramate tool behaves nicely with other language
files in the same repository and it adds **no constraints** to the organization
of your directories and files.

# Create Stacks

When working with Infrastructure as Code it's considered to be a best practice
to split up and organize your IaC into several smaller and isolated stacks.

The stacks are independent configurations that create resources when executed.
Sometimes it's not easy to figure if two resources must be kept in the same
stack or separated but asking the questions below to yourself could help:

- TBD: help please
- Are they related to the same cloud resource?
- Is it acceptable that changes to one resource could affect the other?
- Do they have similar lifecycles? Eg.: destroying one always imply destroying
  the other?

For more information about them, have a look at the [stack](./stack.md)
documentation page.

A stack is just a directory in the repository (even the root of the repository
could be a valid stack directory).
Stacks can have child stacks and stacks can have relationships (explained later
in the [orchestration](#orchestration) section).

Let's create two stacks for deploying a local [NGINX](https://nginx.org/) 
and a [PostgreSQL](https://postgresql.org) containers using the Terraform 
[docker provider](https://registry.terraform.io/providers/kreuzwerker/docker/latest/docs).

But first, let's create a git feature branch for the `nginx` service:

> This is important for understanding the Terramate change detection feature.

```
$ git checkout -b nginx-service
Switched to a new branch 'nginx-service'
```

Terramate comes with a handy `terramate create` command to easily create stacks.

```shell
$ terramate create nginx
Created stack /nginx
```

This command creates the `nginx` directory containing a `stack.tm.hcl` file
similar to the one below:

```hcl
stack {
  name        = "nginx"
  description = "nginx"
  id          = "8b9c6e39-5145-40f1-90f1-67d022b6a6e9"
}
```

The `stack.name` and `stack.description` can be customized with strings that
better document the stack purpose. The `stack.id` is a randomly generated UUID
that must uniquely identify the stack in this repository.
For a complete list of the stack attributes, see the [stack](./stack.md)
documentation page.

Now if you execute `terramate list` you must see your brand new stack listed
in the output:

```shell
$ terramate list
2023-04-08T02:45:01+01:00 WRN repository has untracked files
nginx
```

As the project now has untracked files, Terramate is very picky and warns about
it because you should never deploy infrastructure from code that's not committed
and pushed to the remote git server. This behavior can be customized with the 
`terramate.config.git` config object, see [here](./project-config.md).

In a real world IaC project, only the CI/CD should deploy infrastructure, then
those safeguards are in place to avoid infrastructure being deployed with
temporary, uncommitted, unreviewed files.

For the purpose of this tutorial, let's disable those safeguards locally by
creating a `.gitignored` file called `disable_git_safeguards.tm.hcl` with
content below:

```hcl
terramate {
  config {
    git {
      check_untracked = false
      check_uncommitted = false
    }
  }
}
```

Please, don't forget to `gitignore` this file because those checks must always
be `enabled` in the CI/CD:

```
# .gitignore
disable_git_safeguards.tm.hcl
```

Now the `terramate list` returns:

```shell
$ terramate list
nginx
```

Now let's create docker resources with Terraform.
Drop the file below into the `nginx/main.tf` file:

```hcl
terraform {
  required_providers {
    docker = {
      source  = "kreuzwerker/docker"
      version = "~> 3.0.1"
    }
  }
}

provider "docker" {}

resource "docker_image" "nginx" {
  name         = "nginx:latest"
  keep_locally = false
}

resource "docker_container" "nginx" {
  image = docker_image.nginx.image_id
  name  = "terramate-tutorial-nginx"
  ports {
    internal = 80
    external = 8000
  }
}
```

The Terraform configuration above creates two resources, the `docker_image` and 
the `docker_container` for running a `nginx` service exposed on host port `8000`.

> If your docker daemon is running on a custom port or you use Windows, then the
> "docker" provider need an additional `host` attribute for daemon address.
> On Windows, the config below is commonly needed:
> 
> provider "docker" {
>   host    = "npipe:////.//pipe//docker_engine"
> }


From the root directory, run:

```shell
$ terramate run -- terraform init
```

The command above will execute `terraform init` in all Terramate stacks (just `nginx` stack at this point).

> ![Note](https://cdn-icons-png.flaticon.com/512/427/427735.png)
> You can think of `terramate run -- cmd` as a more robust version of the shell
> script below:
> 
>   ```shell
>   for stack in $(terramate list); do
>     cd $stack;
>     cmd;
>   done
>   ```
>
> But the `terramate run` also pulls `wanted`, computes the correct stack 
> execution order, detect changed stacks, run safeguards, etc.

The Terraform initialization will create the directory `nginx/.terraform` and
the file `nginx/.terraform.lock.hcl`. These files must never be committed to
the version control and it's recommended to be added to the `.gitignore` as well.
Additionally, you should also ignore the `terraform.tfstate` file as it contains
sensitive information.

Example:

```
# .gitignore

# Terramate files
disable_git_safeguards.tm.hcl

# Terraform files
terraform.tfstate*
.terraform
.terraform.lock.hcl
```

Executing `terramate run -- terraform apply` will create the resources.

```shell
$ terramate run -- terraform apply

Terraform used the selected providers to generate the following execution plan. Resource actions are indicated with the following symbols:
  + create

Terraform will perform the following actions:

  # docker_container.nginx will be created
  + resource "docker_container" "nginx" {
      + attach                                      = false
      + bridge                                      = (known after apply)
      + command                                     = (known after apply)
      + container_logs                              = (known after apply)
      + container_read_refresh_timeout_milliseconds = 15000
      + entrypoint                                  = (known after apply)
      + env                                         = (known after apply)
      + exit_code                                   = (known after apply)
      + hostname                                    = (known after apply)
      + id                                          = (known after apply)
      + image                                       = (known after apply)
      + init                                        = (known after apply)
      + ipc_mode                                    = (known after apply)
      + log_driver                                  = (known after apply)
      + logs                                        = false
      + must_run                                    = true
      + name                                        = "terramate-tutorial-nginx"
      + network_data                                = (known after apply)
      + read_only                                   = false
      + remove_volumes                              = true
      + restart                                     = "no"
      + rm                                          = false
      + runtime                                     = (known after apply)
      + security_opts                               = (known after apply)
      + shm_size                                    = (known after apply)
      + start                                       = true
      + stdin_open                                  = false
      + stop_signal                                 = (known after apply)
      + stop_timeout                                = (known after apply)
      + tty                                         = false
      + wait                                        = false
      + wait_timeout                                = 60

      + healthcheck {
          + interval     = (known after apply)
          + retries      = (known after apply)
          + start_period = (known after apply)
          + test         = (known after apply)
          + timeout      = (known after apply)
        }

      + labels {
          + label = (known after apply)
          + value = (known after apply)
        }

      + ports {
          + external = 8000
          + internal = 80
          + ip       = "0.0.0.0"
          + protocol = "tcp"
        }
    }

  # docker_image.nginx will be created
  + resource "docker_image" "nginx" {
      + id           = (known after apply)
      + image_id     = (known after apply)
      + keep_locally = false
      + name         = "nginx:latest"
      + repo_digest  = (known after apply)
    }

Plan: 2 to add, 0 to change, 0 to destroy.

Do you want to perform these actions?
  Terraform will perform the actions described above.
  Only 'yes' will be accepted to approve.

  Enter a value: yes

docker_image.nginx: Creating...
docker_image.nginx: Still creating... [10s elapsed]
docker_image.nginx: Creation complete after 10s [id=sha256:080ed0ed8312deca92e9a769b518cdfa20f5278359bd156f3469dd8fa532db6bnginx:latest]
docker_container.nginx: Creating...
docker_container.nginx: Creation complete after 1s [id=0854270a600861cb72f36d3a78084c240826170601171416bfe3a8e0f1a4547c]

Apply complete! Resources: 2 added, 0 changed, 0 destroyed.
```

Then now the `nginx` service should be running, you can check this with the
`docker ps` command:

```shell
$ docker ps
CONTAINER ID   IMAGE          COMMAND                  CREATED         STATUS         PORTS                  NAMES
0854270a6008   080ed0ed8312   "/docker-entrypoint.…"   2 minutes ago   Up 2 minutes   0.0.0.0:8000->80/tcp   terramate-tutorial-nginx
```

or opening [http://localhost:8000/](https://localhost:8000/) in the browser.

Now let's create the _PostgreSQL_ stack.

```shell
$ terramate create postgresql
Created stack /postgresql
```

Running list now shows:

```shell
$ terramate list
nginx
postgresql
```


TODO: create postgres docker and make both stacks DRY

TODO: configure the www index page of the container using code generation.
```hcl
terraform {
  required_providers {
    docker = {
      source  = "kreuzwerker/docker"
      version = "~> 3.0.1"
    }
  }
}

variable "www_path" {
  description = "Path to the www directory to be mapped into NGINX container"
}

provider "docker" {}

resource "docker_image" "nginx" {
  name         = "nginx:latest"
  keep_locally = false
}

resource "docker_container" "nginx" {
  image = docker_image.nginx.image_id
  name  = "tutorial"
  ports {
    internal = 80
    external = 8000
  }

  volumes {
    host_path      = var.pwd
    container_path = "/usr/share/nginx/html"
  }
}
```