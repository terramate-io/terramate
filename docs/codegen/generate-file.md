# File Generation

Terramate supports the generation of arbitrary text files referencing
[Terramate defined data](../sharing-data.md).

File generation is done using `generate_file`
blocks in [Terramate configuration files](../config-overview.md).

Each `generate_file` block requires a single label that is the path
where the generated file will be saved.
For more details about how code generation use labels check the [Labels Overview](overview.md#labels) docs.

The block has an optional **`context`** attribute which overrides the [generation context]
(overview.md#generation-context).

The **`content`** attribute defines the string that will be written on the file.

The value of the **`content`** has access to different Terramate features 
depending on the `context` defined.

For `context=root` it has access to:

- Terramate Project Metadata references `terramate.root.*` and `terramate.stacks.*`
- [Terramate function](../functions.md#terramate-functions) calls `tm_*(...)`
- Expressions using string interpolation `"${}"`

and for `context=stack` (the default), it has access to everything defined above
for `root` plus the features below:

- Terramate Global references `global.*`
- Terramate Stack Metadata references `terramate.stack.*`

The final evaluated value of the **`content`** attribute **must** be a valid string.

## Generating different file types

### Generating a JSON file

```hcl
generate_file "hello_world.json" {
  content = tm_jsonencode({"hello"="world"})
}
```

### Generating an YAML file

```hcl
generate_file "hello_world.yml" {
  content = tm_yamlencode({"hello"="world"})
}
```

### Generating arbitrary text

It is possible ot use [strings and templates](https://www.terraform.io/language/expressions/strings#strings-and-templates) as known form Terraform.

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

## Hierarchical Code Generation

A `generate_file` block can be defined on any level within a projects hierarchy:
Within a stack or in any parent level up to the root of the projects.

There is no overriding or merging behavior for `generate_file` blocks.
Blocks defined at different levels with the same label aren't allowed, resulting
in failure of the overall code generation process.

This behavior might change in future versions of Terramate.

## Conditional Code Generation

Conditional code generation is achieved by the use of the `condition` attribute.
The `condition` attribute should always evaluate to a boolean.

The file will be generated only if it evaluates to **`true`**.

The default value of the `condition` attribute is `true`.

Any expression that produces a boolean can be used, including references
to globals and function calls. For example:

```hcl
generate_file "file" {
  condition = tm_length(global.list) > 0

  content = "file contents"
}
```

Will only generate the file for stacks that the expression
`tm_length(global.list) > 0` evaluates to true.

Other useful conditions include:

- `tm_can(global.myvariable)` -> generate when `global.myvariable` is set to any value
- `tm_try(global.myboolean, false)` -> generate when `global.myboolean` exists and is`true`.
- `tm_try(global.myvariable != null, false)` -> generate when `global.myvariable` is set and not`null`.

When `condition` is `false` the `generate_file` block won't be evaluated, no file will be created, but any existing file with that name will be removed.

So using `condition = false` will ensure a file is deleted e.g. if previously created by Terramate.
