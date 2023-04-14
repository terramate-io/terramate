# Globals Architecture

Nomenclature:

- `reference` (or `ref`)

A `ref` is a value acessor or query, and it can appear in both sides of a 
globals declaration statement.

The code below:

```
globals {
    a = global.b.c
}
```

has two _references_:

1. `global.a`     (in the left-hand-side)
2. `global.b.c`   (in the right-hand-side)

- Globals statement

A _statement_ is a global value definition and it has two parts, the left-hand-side
(lhs) _ref_ and the right-hand-side (rhs) expression.

```
<ref> = <expression>
```

A `globals` block is interpreted as 0 or more statements.
Example:

```hcl
globals "a" "b" {
    c = {
        d = 1
    }
    z = 2
}
```
is interpreted as the list of statements below:

```
global.a.b.c.d = 1
global.z = 2
```

- The _origin reference_

The _origin_ of a reference is the `globals` attribute that originated the 
statement. Very often it's the same _ref_ as the statement lhs _ref_ itself but
that's not always the case:

Example:

```
globals "a" {
    b = 1
}
```

In this case, it generates the statement below:
```
global.a.b = 1
```
and the _origin reference_ (the attribute that creates it) is `global.a.b`.
But have a look at the example below:

```
globals a {
    b = {
        c = {
            d = 1
        }
    }
}
```
In this case it generates the statements below:

```
global.a.b.c.d = 1
```
but the _origin reference_ is `global.a.b` because that's the attribute that 
originated the internal refs.

## Properties

- The `globals` are blocks that define the single runtime `global` object.
- There could be multiple syntactic `globals` blocks defined in the same directory
as long as their LHS _refs_ are unique.

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
# ref = value
global.a = 1
global.b = 2
global.c = 3
global.z = 4
```

The literal objects are also constructed as ref based assignments:


```hcl
# /globals.tm
globals "a" "b" {
    c = {
        d = 1
    }
}

globals {
    a = {
        b = {
            c = {
                e = 1
                z = 1
            }
        }
    }
}
```

interpreted as:

```
global.a.b.c.e = 1
global.a.b.c.z = 1
global.a.b.c.d = 1
```

- The labels in the `globals` block is a syntax sugar for building a nested _LHS_
`ref`, automatically building the intermediate object keys.

```hcl
globals a b c {
    val = 1
}
```

is interpreted as:

```js
if (!global.a) {
    global.a = {}
}
if (!global.a.b) {
    global.a.b = {}
}
if (!global.a.b.c) {
    global.a.b.c = {}
}

global.a.b.c.val = 1
```

By the same definition above, a labeled globals block without any attributes
just build the intermediate refs:

```hcl
globals "a" "b" {}
```

is interpreted as:

```js
if (!global.a) {
    global.a = {}
}
if (!global.a.b) {
    global.a.b = {}
}
```

- scopes are defined by the directory hierarchy

Each directory defines a new global scope which inherits parent globals.

```hcl
# /root.tm
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
and
```hcl
# /child/grand-child/globals.tm
globals {
    c = global.b
}
```

The code above defines the scope tree below:

```hcl
scope = {
    global = {
        a = 1
    }
    scopes = {
        child = {
            global = {
                b = global.a
            },
            scopes = {
                "grand-child": {
                    global = {
                        c = global.b
                    }
                }
            }
        }
    }
}
```

The `grand-child` scope inherits the `child` and `root` scopes.
The `child` scope inherits the `root` scope.

- implicit order of evaluation 

The order of evaluation of global values are implicitly defined by their dependencies and _origin ref_ size (ie `global.a` evaluates before `global.a.b`).

Case 1:

```hcl
# /globals.tm
globals {
    b = global.a
    a = 1
}
```

As `global.b` depends on `global.a`, then the order of evaluation is:

```
global.a = 1
global.b = global.a
```

When multiple global lhs _references_ target the same object tree, then
statements with smaller _origin reference_ evaluates first:

Example:

```hcl
# /globals.tm
globals "a" "b" {
    c = {
        d = 1
    }
}

globals {
    a = {
        b = {
            c = {
                e = 1
                z = 1
            }
        }
    }
}
```

interpreted as:

```
global.a.b.c.e = 1  # origin reference is global.a
global.a.b.c.z = 1  # origin reference is global.a
global.a.b.c.d = 1  # origin reference is global.a.b.c
```

 # Evaluation

When evaluating an expression, just the _referenced_ globals are evaluated.
If a target _ref_ is not found in the current scope, then it's looked up in the 
parent scope until root is reached or a _origin reference_ is found which is a 
subpath of the target _ref_.

Case 1: _ref_ is in the same scope (no dependency).

target ref: `global.a.b` in `/child` scope

```hcl
# /root.tm
globals {
    a = {
        b = 1
    }
    b = 1
    c = {
        b = 1
    }
}
```

```hcl
# /child/globals.tm
globals a {
    b = 2
}
```

evaluates to `2` and only the statement `global.a.b = 2` from the `/child` scope is evaluated.

Case 2: _ref_ is in the same scope (with dependencies).

target ref: `global.a.b` in `/child` scope

```hcl
# /root.tm
globals "a" {
    b = 1
}

globals {
    c = 2
}
```

```hcl
# /child/globals.tm
globals "a" {
    b = global.c
}
```
evaluates to `2` because _origin ref_ `global.a.b` is found in the same scope,
then `global.c` is lookup in parent scope.

Case 3: _ref_ is in the parent scope (no deps)

target ref: `global.a.b` in `/child` scope

```hcl
# /root.tm
globals "a" {
    b = 1
}
```

```hcl
# /child/globals.tm
```

evaluates to `1` and only the `global.a.b = 1` from `/` is evaluated.

Case 4: lazy evaluation

target ref: `global.a.b` from `/child`.

```hcl
# /root.tm
globals "a" {
    b = global.c
}

globals {
    c = 1
}
```

```hcl
# /child/globals.tm
globals {
    c = 2
}
```

It evaluates to `2` and only `global.a.b = global.c` from `/` and `global.c` from `/child` are evaluated.
