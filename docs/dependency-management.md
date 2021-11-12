# Dependency Management

Sometimes stacks are completely independent from each other, but on
certain occasions it may happen that infrastructure that is created
by stack **A** is required by stack **B**, like using the outputs
of stack **A** as inputs for stack **B**.

This can be done through data sources (preferable, when available) or
by [loading the state](https://www.terraform.io/docs/language/state/remote-state-data.html)
of another stack. Independent on how you solve the problem, you need
an explicit way to communicate that changes on **stack A** can affect
**stack B**, so the order of execution of the stacks should always be:

* 1 - **stack A**
* 2 - **stack B**

To help with that terrastack provides a way to explicit declare
dependencies between stacks.

## Declaring Dependencies

Dependencies are declared inside the **stack** block using the
parameter **dependencies** which is a set of strings (set(string)),
where each string is a reference to a another stack.

For example, lets say we have a project organized like this:

```
.
├── stack-a
│   ├── main.tsk
│   └── version.tsk
└── stack-b
    ├── main.tsk
    └── version.tsk
```

And **stack-a/main.tsk** looks like:

```
stack {
    // variables defintions
}
```

Which doesn't depend on anything,
and then we have **stack-b/main.tsk**:

```
stack {
    dependencies = [
        "../stack-a"
    ]
    // variables definitions
}
```

This means that **stack-b** depends on **stack-a**.
The expression of this dependency impacts order of
execution of the stacks and also how change detection
works across stacks, which is defined along this doc,
but as far as defining dependencies goes, it is this easy.


## Order of Execution

## Parallel Execution

## Inspecting the Dependency Graph

## What About Version Selection ?

## What About Cycles ?
