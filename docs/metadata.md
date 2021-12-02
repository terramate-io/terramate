# Metadata

Terrastack provides a set of well defined metadata that can be
accessed through the variable namespace **terrastack**.

This can be referenced from any terrastack code to reference
information like the path of the stack that is being evaluated.


## terrastack.path (string) 

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

* terrastack.path for **stack-a** = /stacks/stack-a
* terrastack.path for **stack-b** = /stacks/stack-b

Inside the context of a project **terrastack.path** can
uniquely identify stacks.


## terrastack.name (string) 

Name of the stack.

Given this stack layout (from the root of the project):

```
.
└── stacks
    ├── stack-a
    └── stack-b
```

* terrastack.name for **stack-a** = stack-a
* terrastack.name for **stack-b** = stack-b
