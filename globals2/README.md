# Globals Architecture

Nomenclature:

- `reference` (or `ref`)

A `ref` is a value acessor or query.

Globals properties:

- multiple syntactic `globals` blocks defined in the same directory are interpreted
as a single `globals` block with all attributes merged, given that they don't
conflict.

The configuration below:

```hcl
globals {
    a = 1
}

globals {
    b = 2
    c = 3
}

globals {
    z = 4
}
```

is interpreted as:

```
global.a = 1
global.b = 2
global.c = 3
global.z = 4
```

- The labels in the `globals` block is a syntax for a `LHS` (Left Hand Side)
reference (`ref`)

```hcl
globals obj a b {
    c = 1
}
```

is interpreted as:

```
global.obj.a.b.c = 1
```

with the implicit creation of the object keys `obj.a.b` if they are not defined
anywhere else.

An empty labeled globals block is a syntax for creating an empty object at the
target labels spec:

```hcl
globals "obj" "test" {

}
```

is interpreted as:

```
global.obj.test = {}
```

- scopes are defined by the directory hierarchy

Each directory defines a new global scope which inherits parent globals.

```hcl
# /globals.tm

globals {
    a = 1
}
```
and
```hcl
# /child/globals.tm

globals {
    b = global.a
}
```

- implicit order of evaluation 

The evaluation of globals are implicitly defined by their dependencies and
scope location.

Examples:

```hcl
# /globals.tm
globals {
    
}
```

TO BE CONTINUED
