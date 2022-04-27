# File Generation

Terramate supports the generation of arbitrary text files referencing 
[Terramate defined data](../sharing-data.md).

File generation is done using `generate_file`
blocks in [Terramate configuration files](../config-overview.md).

The block **must** have a single label, that will be used to determine the
name of the generated file. Inside the block the **content** attributes defines
the string that will be written on the file.

The definition of the **content** attribute may include:

* Terramate Global references
* Terramate Metadata references
* Terramate function calls
* Expressions using string interpolation

The final evaluated **content** attribute **must** be a string.

Multiple `generate_file` blocks with the same label/filename will
result in an error.

Here is an example on how a JSON file can be created:

```hcl
generate_file "hello_world.json" {
  content = tm_jsonencode({"hello"="world"})
}
```

Generating an YAML file:

```hcl
generate_file "hello_world.json" {
  content = tm_yamlencode({"hello"="world"})
}
```

Generating arbitrary text using
[strings and templates](https://www.terraform.io/language/expressions/strings#strings-and-templates):

```hcl
generate_file "hello_world.json" {
  content = <<EOT
whatever text format you want, here is a global reference:

a = ${global.reference}

and now a metadata reference:

b = ${terramate.path}
EOT
}
```

## Hierarchical Code Generation

File generation can be defined anywhere inside a project, from a specific
stack, which defines code generation only for the specific stack, to parent dirs
or even the project root, which then generates the file for multiple stacks.

There is no overriding or merging behavior for `generate_file` blocks.
Blocks defined at different levels with the same label aren't allowed, resulting
in failure for the overall code generation process.
