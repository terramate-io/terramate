# File Generation

Terramate supports the generation of arbitrary text files referencing 
[Terramate defined data](../sharing-data.md).

File generation is done using `generate_file`
blocks in [Terramate configuration files](../config-overview.md).

The block **must** have a single label, that will be used to determine the
name of the generated file. Inside the block, the **content** attributes define
the string that will be written on the file.

The definition of the **content** attribute may include:

* Terramate Global references
* Terramate Metadata references
* Terramate function calls
* Expressions using string interpolation

The final evaluated **content** attribute **must** be a string.

Multiple `generate_file` blocks with the same label/filename will
result in an error.

Here is an example of how a JSON file can be created:

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

and a metadata reference:

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
in failure of the overall code generation process.


## Conditional Code Generation

Conditional code generation is achieved by the use of the `condition` attribute.
The `condition` attribute should always evaluate to a boolean. The code will
be generated only if it evaluates to **true**.

Any expression that produces a boolean can be used, including references
to globals and function calls. For example:

```hcl
generate_file "file" {
  condition = global.generate_file

  content = "file contents"
}
```

Will only generate the file for stacks that define the global `generate_file` to true.
