# Sharing Data

In order to keep your code [DRY](https://en.wikipedia.org/wiki/Don%27t_repeat_yourself)
it is important to have an easy and safe way to define data once and share it
across different stacks.

This is done on Terramate using globals and metadata. Globals are defined by
the user, similar to how you would define locals in Terraform, and metadata
is provided by Terramate itself.

Terramate globals and metadata are integrated with Terraform using code
generation, you can check it into more details [here](codegen/overview.md).

# Globals

Globals provide a way to define information that can be re-used
across stacks with a clear hierarchical/merge semantic.

Defining globals is fairly straightforward, you just need to
add a **globals** block to your [Terramate configuration file](config-overview.md):

```hcl
globals {
  env = "staging"
  values = [ 1, 2, 3 ]
}
```

And you can reference them on [code generation](codegen/overview.md):

```hcl
generate_hcl "backend.tf" {
  content {
    backend "type" {
      param = "value-${global.env}"
    }
  }
}

globals {
  env = "staging"
}
```

Globals can reference [metadata](metadata.md):

```hcl
globals {
  extended_stack_path = "extended/${terramate.path}"
}
```

And also reference other globals:

```hcl
globals {
  info = "something"
  extended_stack_path = "${global.info}/extended/${terramate.path}"
}
```

Globals can be defined on multiple Terramate files, lets call this
set of files in a specific directory a **configuration**. Given
this definition:

* A project has multiple configurations, one for each of its directories. 
* The most specific configuration is the stack directory.
* The most general configuration is the project root directory.
* Globals can't be redefined in the same configuration.
* Globals can be redefined in different configurations.
* Globals can reference globals from other configurations.

Each stack will have its globals defined by loading them from
the stack directory and all the way up until the project root
is reached. This is called the stack globals set.

For globals being redefined on different configurations we follow
a very simple merge strategy to build each stack globals set:

* Globals with different names are merged together.
* Globals with same names: more specific configuration replaces the general one.

Lets explore a little further with an example.
Given a project structured like this:

```
.
└── stacks
    ├── stack-1
    │   └── terramate.tm.hcl
    └── stack-2
        └── terramate.tm.hcl
```

The configurations available, from more specific to more general, for `stack-1` are:

* stacks/stack-1
* stacks
* . (the project root dir)

To create globals that will be available for all stacks int the entire project
just add a [Terramate configuration file](config-overview.md) on the project
root with some useful globals:

```hcl
globals {
  project_name = "awesome-project"
  useful       = "useful"
}
```

Now any stack on the project can reference these globals on their
[Terramate configuration](config-overview.md).

Now lets say one of the stacks wants to add more globals, to do
so we can add globals on the stack configuration by creating the file
`stacks/stack-1/globals.tm.hcl`:

```hcl
globals {
  stack_data = "some specialized stack-1 data"
}
```

Now `stacks/stack-1` globals set is:

```
project_name = "awesome-project"
useful       = "useful"
stack_data   = "some specialized stack-1 data"
```

And for `stacks/stack-2`:

```
project_name = "awesome-project"
useful       = "useful"
```

Now lets redefine a global on `stacks/stack-1`.
We do that by changing `stacks/stack-1/globals.tm.hcl`:

```hcl
globals {
  useful     = "overriden by stack-1"
  stack_data = "some specialized stack-1 data"
}
```

Now `stacks/stack-1` globals set is:

```
project_name = "awesome-project"
useful       = "overriden by stack-1"
stack_data   = "some specialized stack-1 data"
```

And for `stacks/stack-2` it remains:

```
project_name = "awesome-project"
useful       = "useful"
```

Overriding happens at the global name level, so objects/maps/lists/sets
won't get merged, they are completely replaced by the most
specific configuration with the same global name.

Lets say we add this to our project root configuration:

```hcl
globals {
  project_name = "awesome-project"
  useful       = "useful"
  object       = { field_a = "field_a", field_b = "field_b" }
}
```

And redefine it on `stacks/stack-1/globals.tm.hcl`:

```hcl
globals {
  object = { field_a = "overriden_field_a" }
}
```

Now `stacks/stack-1` globals set is:

```
project_name = "awesome-project"
useful       = "useful"
object       = { field_a = "overriden_field_a" }
```

And for `stacks/stack-2`:

```
project_name = "awesome-project"
useful       = "useful"
object       = { field_a = "field_a", field_b = "field_b" }
```

## Lazy Evaluation

So far we described how globals on different configurations are merged.
Given that globals can reference other globals and Terramate metadata it is
important to be clear about how/when evaluation happens. 

Globals are lazily evaluated. The whole process per stack can
be described on this order:

* On each configuration, starting on the stack, globals definitions are loaded.
* Merge strategy is applied as configurations are loaded.
* All merging is done and the globals set is defined for a stack.
* The globals set is evaluated.

This means that globals at the root configuration of a project can reference
globals that are going to be defined only at a more specific configuration
(potentially the stack itself). 

Overall globals are evaluated lazily, actual evaluation only happens after
all globals have been loaded and merged from all configurations defined
for a stack (stack + its parents dir until project root).

So on the project root you can reference globals that will only be defined
later when a stack defines it, just as on a stack you can reference globals
that are defined on the project root (or any parent dir).

Given a project organized like this:

```
.
└── envs
    ├── prod
    │   └── stack
    │       └── terramate.tm.hcl
    └── staging
        └── stack
            └── terramate.tm.hcl
```

We can define a single version of a [backend configuration](backend-config.md)
for all envs referencing env + stack specific information at **envs/terramate.tm.hcl**:

```hcl
terramate {
  backend "gcs" {
    bucket = global.gcs_bucket
    prefix = global.gcs_prefix
  }
}

globals {
  gcs_bucket = "prefix-${global.env}"
  gcs_prefix = terramate.path
}
```

Neither at **envs** or at the parent dir is **global.env** defined. Any subdir
until the stack is reached can define it (or override it if it is already defined),
final values are evaluated when reaching the stack itself.

We can define **global.env** once per env.

For production **envs/prod/terramate.tm.hcl**:

```hcl
globals {
  env = "prod"
}
```

For staging **envs/staging/terramate.tm.hcl**:

```hcl
globals {
  env = "staging"
}
```

Given this setup, for  the stack **/envs/prod/stack**
we have the following globals defined:

```
global.env        = "prod"
global.gcs_bucket = "prefix-prod"
global.gcs_prefix = "/envs/prod/stack"
```

And the following backend configuration:

```hcl
terramate {
  backend "gcs" {
    bucket = "prefix-prod"
    prefix = "/envs/prod/stack"
  }
}
```

And for the stack **/envs/staging/stack**:

```
global.env        = "staging"
global.gcs_bucket = "prefix-staging"
global.gcs_prefix = "/envs/staging/stack"
```

And the following backend configuration:

```hcl
terramate {
  backend "gcs" {
    bucket = "prefix-staging"
    prefix = "/envs/staging/stack"
  }
}
```

# Metadata

Terramate provides a set of well defined metadata that can be
accessed through the variable namespace **terramate**.

This can be referenced from any terramate code to reference
information like the path of the stack that is being evaluated.

To see all metadata available on your project run:

```
terramate metadata
```

## terramate.path (string) 

Absolute path of the stack.  The path is relative to the project
root directory, not the host root directory. So it is absolute
on the context of the entire project.

Given this stack layout (from the root of the project):

```
.
└── stacks
    ├── stack-a
    └── stack-b
```

* terramate.path for **stack-a** = /stacks/stack-a
* terramate.path for **stack-b** = /stacks/stack-b

Inside the context of a project **terramate.path** can
uniquely identify stacks.


## terramate.name (string) 

Name of the stack.

Given this stack layout (from the root of the project):

```
.
└── stacks
    ├── stack-a
    └── stack-b
```

* terramate.name for **stack-a** = stack-a
* terramate.name for **stack-b** = stack-b


## terramate.description (string) 

The description of the stack, if it has any. The default value is an empty string
if undefined.

To define a description for a stack just add a **description**
attribute to the **stack** block:

```hcl
stack {
  description =  "some description"
}
```
