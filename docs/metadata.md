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
