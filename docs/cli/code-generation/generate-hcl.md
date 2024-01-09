---
title: Terraform, OpenTofu and HCL Code Generation
description: Learn how to use Terramate to generate Terraform and OpenTofu configurations.
---

# HCL Code Generation

Terramate supports the generation of arbitrary HCL code such as Terraform, OpenTofu and other HCL configurations,
referencing data such as [Variables](./variables/index.md) and [Metadata](./variables/metadata.md).

## The `generate_hcl` block

HCL code generation is done using `generate_hcl` blocks in Terramate configuration files.
References to Terramate globals and metadata are evaluated, but any other reference is just transported to the generated code
(For details, please see [partial evaluation](./index.md#partial-evaluation)).

```hcl
# example.tm.hcl
generate_hcl "backend.tf" {
  content {
    backend "local" {}
  }
}
```

The label of the `generate_hcl` block names the file that will be generated within a stack.
[Terramate Variables](./variables/index.md) (`let`, `global`, and `terramate` namespaces) and all [Terramate Functions](./functions/index.md)
are supported when defining labels. For more details about how code generation uses labels check the [Labels Overview](./index.md#labels) docs.

### Argument reference of the `generate_hcl` block

- `content` *(required block)* The `content` block defines the HCL code that will be generated as file content. 
  It supports block definitions, attributes and expressions. Terramate Variables and Terramate Functions can be used and will be interpolated during code generation. 

  The following variable namespaces are available within the `content` block:
  - [`terramate`](./variables/metadata.md)
  - [`global`](./variables/globals.md)
  - [`let`](./variables/lets.md)
  
  ```hcl
  content {
    backend "local" {}
  }
  ```

  In addition, the special block [`tm_dynamic`](#the-tm_dynamic-block) is available to generate dynamic content.
  Any references to functions, variables or blocks that Terramate is unaware of will be rendered as-is.
  See [partial code generation](#partial-evaluation) for details.

- `lets` *(optional block)* One or more [`lets`](./variables/lets.md) blocks can be used to define [Lets variables](./variables/lets.md)
  that can be used in other arguments within the `generate_hcl` block and in the `content` block and are only available 
  inside the current `generate_hcl` block.
  
  ```hcl
  lets {
    temp_a_plus_b = global.a + global.b
  }
  ```
  
  ::: tip
  Use Lets over Global variables whenever you want to provide computed variables available inside the current `generate_hcl` block only.
  :::

- `stack_filter` *(optional block)* Stack filter allow to filter stacks where the code generation should be executed.
Currently, only path-based filters are available but tag-based filters are coming soon. Stack filters support neither
Terramate Functions nor Terramate Variables. For advanced filtering of stacks based on additional conditions and complex
expressions please use `condition` argument. `stack_filter` blocks have precedence over `conditions` and will be executed
first for performance reasons. A stack will only be selected for code generation if any `stack_filter` is `true` and the
`condition` is `true` too.

  Each `stack_filter` block supports one or more of the following arguments. When specifying more attributes, all need to be
  `true` to mark the `stack_filter` block as `true`.
    - `project_paths` *(optional list of strings)* A list of patterns matched against the absolute project path of the stack.
    The patterns support globbing but no regular expressions. Any matched path in the list will mark the project path filter as `true`.
    - `repository_paths` *(optional list of strings)* A list of patterns matched against the absolute repository path of the stack.
    The patterns support globbing but no regular expressions. Any matched path in the list will mark the repository path filter as `true`.

    ```hcl
    stack_filter {
      project_paths = [
        "/path/to/specific/stack", # match exact path
        "/path/to/some/stacks/*",  # match stacks in a directory
        "/path/to/many/stacks/**", # match all stacks within a tree
      ]
    }
    ```

- `condition` *(optional boolean)* The `condition` attribute supports any expression that renders to a boolean.
Terramate Variables (`let`, `global`, and `terramate` namespaces) and all Terramate Functions are supported.
Variables are evaluated with the stack context. For details, please see [Lazy Evaluation](./index.md#lazy-evaluation).
If the condition is `true` and any `stack_filter` (if defined) is `true` the stack is selected for generating the code.
As evaluating the condition for multiple stacks can be slow, using `stack_filter` for path-based generation is recommended.

  ```hcl
  condition = tm_anytrue([
     tm_contains(terramate.stack.tags, "my-tag"), # only render if tag is set
     tm_try(global.render_stack, false),          # only render if `render_stack` is `true`
  ])
  ```

- `assert` *(optional block)* One or more `assert` blocks can be used to prevent wrong configurations in code generation
assertion can be set to guarantee all preconditions for generating code are satisfied.
Each `assert` block supports the following arguments:
  - `assertion` *(required boolean)* When the boolean expression is `false` the assertion is triggered and the `message`
  is printed to the user. Terramate Variables (`let`, `global`, and `terramate` namespaces) and all Terramate Functions are supported.
  - `message` *(required string)* A descriptive message to present to the user to inform about the causes that made an assertion fail.
  Terramate Variables (`let`, `global`, and `terramate` namespaces) and all Terramate Functions are supported.
    - `warning` *(optional boolean)* When set to `true` the code generation will not fail, but a warning is issued to the user.
    Default is `false`. Terramate Variables (`let`, `global`, and `terramate` namespaces) and all Terramate Functions are supported.
  ```hcl
  assert {
    assertion = tm_can(global.is_enabled)
    message   = "'global.is_enabled' needs to be set to either true or false"
  }
  ```
    
<!-- ### Complete Example -->
    
## The `tm_dynamic` block

::: info
The `tm_dynamic` block is only supported within the `content` block of a `generate_hcl` block.
:::

Additionally, a `labels` attribute can be provided for generating the block's
labels. Example:

```hcl
globals {
  values = ["a", "b", "c"]
}

generate_hcl "file.tf" {
  content {
    tm_dynamic "block" {
      for_each = global.values
      iterator = value
      labels   = ["some", "labels", value.value]

      content {
        key   = value.key
        value = value.value
      }
    }
  }
}
```

which generates:

```hcl
block "some" "labels" "a" {
  key   = 0
  value = "a"
}
block "some" "labels" "b" {
  key   = 1
  value = "b"
}
block "some" "labels" "c" {
  key   = 2
  value = "c"
}
```

The `labels` must evaluate to a list of strings, otherwise it fails.

The `tm_dynamic` content block only evaluates the Terramate variables/functions,
everything else is just copied as is to the final generated code.

The same goes when using `attributes`:

```hcl
globals {
  values = ["a", "b", "c"]
}

generate_hcl "file.tf" {
  content {
    tm_dynamic "block" {
      for_each = global.values
      iterator = value

      attributes = {
        attr  = "index: ${value.key}, value: ${value.value}"
        attr2 = not_evaluated.attr
      }
    }
  }
}
```

Also generates a `file.tf` file like this:

```hcl
block {
  attr = "index: 0, value: a"
  attr2 = not_evaluated.attr
}

block {
  attr = "index: 1, value: b"
  attr2 = not_evaluated.attr
}

block {
  attr = "index: 2, value: c"
  attr2 = not_evaluated.attr
}
```

The `for_each` attribute is optional. If it is not defined then only a single block
will be generated and no iterator will be available on block generation.

The `tm_dynamic` block also supports an optional `condition` attribute that must
evaluate to a boolean. When not defined it is assumed to be true. If the `condition`
is false the `tm_dynamic` block is ignored, including any of its nested `tm_dynamic`
blocks. No other attribute of the `tm_dynamic` block is evaluated if the `condition`
is false, so it is safe to use it like this:

```hcl
generate_hcl "file.tf" {
  content {
    tm_dynamic "block" {
      for_each  = global.values
      condition = tm_can(global.values)
      iterator  = value

      attributes = {
        attr  = "index: ${value.key}, value: ${value.value}"
        attr2 = not_evaluated.attr
      }
    }
  }
}
```

And if `global.values` is undefined the block is just ignored.

## Filter-based Code Generation

To only generate HCL code for stacks matching specific criteria, a `stack_filter`
block can be added within a `generate_hcl` block.

The following filter attributes for path filtering are supported:

* `project_paths`: Match any of the given stack paths relative to the project root.
* `repository_paths`: Match any of the given stack paths relative to the repository root.

Stack paths support glob-style wildcards:
* `*` matches any sequence of characters until the next directory separator (`/`).
* `**` matches any sequence of characters.

Unless a path starts with `*` or `/`, it is implicitly prefixed with `**/`.

If multiple attributes are set per `stack_filter`, _all_ of them must match.

If multiple `stack_filter` blocks are added, at least one must match.

Here's an example:

```hcl
generate_hcl "file" {
  stack_filter {
    project_paths = ["networking/**"]
  }
  content {
    resource "networking_resource" "name" {
      # ...
    }
  }
}
```
This generates a file containing a networking resource only for stacks located within a
directory named `networking`. The implicitly added prefix `**/` means that this directory
can be located anywhere in our project. The suffix `/**` means that the stack can be in
any nested sub-directory of a `networking` directory.

If we change the attribute to `project_paths = ["/networking/*"]`,
it only matches stacks that are directly in a `networking` directory located at
the project root level.

If more complex logic is required to decide if a file should be generated,
see the `condition` attribute described in the next section.


## Conditional Code Generation

Conditional code generation is achieved by the use of the `condition` attribute.
The `condition` attribute should always evaluate to a boolean. The file will
be generated only if it evaluates to **true**.

If the `condition` attribute is absent then it is assumed to be true.

Any expression that produces a boolean can be used, including references
to globals and function calls. For example:

```hcl
generate_hcl "file" {
  condition = tm_length(global.list) > 0
  content {
    locals {
      list = global.list
    }
  }
}
```

Will only generate the file for stacks that the expression
`tm_length(global.list) > 0` evaluates to true.

When `condition` is false the `content` block won't be evaluated.

- A single label is required to define the type of the block to be dynamically generated. If the block needs to specify
any labels the `labels` argument can be used to populate any number of labels.
- `labels` **(optional list of string)** Define any number of labels the block shall be generated with.
Terramate Variables (`let`, `global`, and `terramate` namespaces) and all Terramate Functions are supported when defining labels.
In addition, the `iterator` namespace is available which defaults to the label of the block being generated but can be renamed by using the `iterator` argument.
- `content` **(optional block)** The `content` block is optional when `attributes` are defined. It supports the same features
as the `generate_hcl.content` block. Terramate Variables (`let`, `global`, and `terramate` namespaces) and all
Terramate Functions are supported when defining labels. In addition, the `iterator` namespace is available which
defaults to the label of the block being generated but can be renamed by using the `iterator` argument.
- `attributes` **(optional map)** The `attributes` argument specifies a map of attributes that shall be rendered inside the
generated block. Those `attributes` are merged with attributes and blocks defined in the `content` block, but they can not
conflict, meaning any given attribute can either defined in `attributes` or `content` but not in both.
Terramate Variables (`let`, `global`, and `terramate` namespaces) and all Terramate Functions are supported when defining labels.
In addition, the `iterator` namespace is available which defaults to the label of the block being generated but can be
renamed by using the `iterator` argument.
- `for_each` **(optional list or map of any type)** The `for_each` argument provides the complex list of values to iterate over.
In each iteration, the `iterator` will be populated with a `value` of the current element. The element is accessible using
the `iterator` namespace and defaults to the label of the block being generated. The value can be accessed with the `value` field.
- `iterator` **(optional string)** The `iterator` sets the name of a temporary variable namespace that represents the current
element of the complex value defined in `for_each`. If omitted, the name of the variable defaults to the label of the `dynamic` block.
- `condition` **(optional boolean)** Instead of using the `for_each` the `condition` argument can be used for triggering
generating the block based on an expression. Terramate Variables (`let`, `global`, and `terramate` namespaces) and
all Terramate Functions are supported when defining labels.

## Partial Evaluation

A partial evaluation strategy is used when generating HCL code.
This means that you can generate code with unknown references/function calls
and those will be copied verbatim to the generated code.

Let's assume we have a single global as Terramate data:

```hcl
globals {
  terramate_data = "terramate_data"
}
```

And we want to mix those Terramate references with Terraform
references, like locals, vars, outputs, etc.
All we have to do is define our `generate_hcl` block like this:


```hcl
generate_hcl "main.tf" {
  content {
    resource "myresource" "name" {
      count = var.enabled ? 1 : 0
      data  = global.terramate_data
      path  = terramate.path
      name  = local.name
    }
  }
}
```

And it will generate the following `main.tf` file:

```hcl
resource "myresource" "name" {
  count = var.enabled ? 1 : 0
  data  = "terramate_data"
  path  = "/path/to/stack"
  name  = local.name
}
```

The `global.terramate_data` and `terramate.path` references were evaluated,
but the references to `var.enabled` and `local.name` were retained as is,
hence the partial evaluation.

Function calls are also partially evaluated. Any unknown function call
will be retained as is, but any function call starting with the prefix
`tm_` is considered a Terramate function and will be evaluated.
Terramate function calls can only have as parameters Terramate references
or literals.

For example, given:

```hcl
generate_hcl "main.tf" {
  content {
    resource "myresource" "name" {
      data  = tm_upper(global.terramate_data)
      name  = upper(local.name)
    }
  }
}
```

This will be generated:

```hcl
resource "myresource" "name" {
  data  = "TERRAMATE_DATA"
  name  = upper(local.name)
}
```

If one of the parameters of an unknown function call is a Terramate
reference the value of the Terramate reference will be replaced on the
function call.

This:

```hcl
generate_hcl "main.tf" {
  content {
    resource "myresource" "name" {
      data = upper(global.terramate_data)
      name = upper(local.name)
    }
  }
}
```

Generates:

```hcl
generate_hcl "main.tf" {
  content {
    resource "myresource" "name" {
      data  = upper("terramate_data")
      name  = upper(local.name)
    }
  }
}
```

Currently, there is no partial evaluation of `for` expressions.
Referencing Terramate data inside a `for` expression will result
in an error (`for` expressions with unknown references are copied as is).
