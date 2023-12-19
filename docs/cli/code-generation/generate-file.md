---
title: Generate Files
description: Learn how to use Terramate to generate files such as JSON or YAML.
---

# File Code Generation

Terramate supports the generation of arbitrary files such as JSON or YAML referencing data such as
[Variables](./variables/index.md) and [Metadata](./variables/metadata.md).

## The `generate_file` block

File code generation is done using `generate_file` blocks in Terramate configuration files.
References to Terramate globals and metadata are evaluated.

```hcl
generate_file "hello_world.json" {
  content = tm_jsonencode({"hello" = "${global.world}"})
}
```

The label of the `generate_file` block names the file that will be generated.
Terramate Variables (`let`, `global`, and `terramate` namespaces) and all [Terramate Functions](./functions/index.md)
are supported when defining labels. For more details about how code generation uses labels check the [Labels Overview](./index.md#labels) docs.


### Argument reference of the `generate_file` block

- `context` *(optional string)* The `context` attributes that override the [generation](./index.md#generation-context) context](index.md#generation-context)
- `content` *(required string)* The `content` argument defines the string that will be generated as the content of the file.
  The value of the **`content`** has access to different Terramate features
  depending on the `context` defined.

  For `context=root` it has access to:

  - Terramate Project Metadata references `terramate.root.*` and `terramate.stacks.*`
  - [Terramate function](./functions/index.md) calls `tm_*(...)`
  - Expressions using string interpolation `"${}"`

  and for `context=stack` (the default), it has access to everything that `root` has plus the features below:

  - Terramate Global references `global.*`
  - Terramate Stack Metadata references `terramate.stack.*`

  The final evaluated value of the **`content`** attribute **must** be a valid string.


  ```hcl
  content = <<-EOF
    Hello World!
  EOF
  ```

- `lets` *(optional block)* One or more `lets` blocks can be used to define [temporary variables](./variables/lets.md)
  that can be used in other arguments within the `generate_file` block and in the `content` block.

  ```hcl
  lets {
    temp_a_plus_b = global.a + global.b
  }
  ```

- `stack_filter` *(optional block)* Stack filter allow to filter stacks where the code generation should be executed.
  Currently, only path-based filters are available but tag-based filters are coming soon. Stack filters do neither support
  Terramate Functions nor Terramate Variables. For advanced filtering of stacks based on additional conditions and complex
  expressions please use `condition` argument. `stack_filter` blocks have precedence over `conditions` and will be executed
  first for performance reasons. A stack will only be selected for code generation if any `stack_filter` is `true` and the
  `condition` is `true` too.

  Each `stack_filter` block supports one or more of the following arguments. When specifying more attributes, all need to
  be `true` to mark the `stack_filter` block as `true`.
  - `project_paths` *(optional list of strings)* A list of patterns matched against the absolute project path of the stack.
  The patterns support globbing but no regular expressions. Any matched path in the list will mark the project path filter as `true`.
  - `repository_paths` *(optional list of strings)* A list of patterns matched against the absolute repository path of the
  stack. The patterns support globbing but no regular expressions. Any matched path in the list will mark the repository path filter as `true`.

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
  Variables are evaluated with the stack context (see  Lazy Evaluation)
  If the condition is `true` and any `stack_filter` (if defined) is `true` the stack is selected for generating the code.
  As evaluating the condition for multiple stacks can be slow, using `stack_filter` for path-based generation is recommended.

  ```hcl
  condition = tm_anytrue([
     tm_contains(terramate.stack.tags, "my-tag"), # only render if tag is set
     tm_try(global.render_stack, false), # only render if `render_stack` is `true`
  ])
  ```

- `assert` *(optional block)* One or more `assert` blocks can be used to prevent wrong configurations in code generation
  assertion can be set to guarantee all preconditions for generating code are satisfied.
  Each `assert` block supports the following arguments:
    - `assertion` *(required boolean)* When the boolean expression is `false` the assertion is triggered and the
    `message` is printed to the user. Terramate Variables (`let`, `global`, and `terramate` namespaces) and all
    Terramate Functions are supported.
    - `message` *(required string)* A descriptive message to present to the user to inform about the causes that made an
    assertion fail. Terramate Variables (`let`, `global`, and `terramate` namespaces) and all Terramate Functions are supported.
    - `warning` (optional boolean) When set to `true` the code generation will not fail, but a warning is issued to the user.
    Default is `false`. Terramate Variables (`let`, `global`, and `terramate` namespaces) and all Terramate Functions are supported.

## Generating different file types

### Generating a JSON file

```hcl
generate_file "hello_world.json" {
  content = tm_jsonencode({"hello"="world"})
}
```

### Generating a YAML file

```hcl
generate_file "hello_world.yml" {
  content = tm_yamlencode({"hello"="world"})
}
```

### Generating arbitrary text

It is possible to use [strings and templates](https://www.terraform.io/language/expressions/strings#strings-and-templates) as known from Terraform.

```hcl
generate_file "hello_world.json" {
  content = <<-EOT
    whatever text format you want, here is a global reference:

    a = ${global.reference}

    and a metadata reference:

    b = ${terramate.path}
  EOT
}
```
