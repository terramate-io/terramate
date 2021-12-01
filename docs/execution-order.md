# Execution Ordering

Sometimes stacks are completely independent of each other, but on
certain occasions it may happen that infrastructure that is created
by **stack A** is required by **stack B**, like using the outputs
of **stack A** as inputs for **stack B**.

This can be done through data sources (preferable, when available) or
by [loading the state](https://www.terraform.io/docs/language/state/remote-state-data.html)
of another stack (or or even an implicit dependency like hard coding the name/ID).

Independent on how you approach the problem, you need
an explicit way to communicate that changes on **stack A** affect execution of
**stack B**, so the order of execution of the stacks should always be:

* 1 - **stack A**
* 2 - **stack B**

To help with that terrastack provides a way to explicit declare
the desired order of execution between stacks.

It is important to note that we are talking strictly about execution
order, not hard dependencies, ordering is imposed on top of selected
stacks, and stacks are only selected if they have changes on it,
so ordering definition never selects an unchanged stack.


## Defining Order Of Execution

Order of execution is declared inside the **stack** block using the
parameters **before** and **after**. 

Each is a set of strings (**set(string)**),
where each string is a reference to another stack.

Those two settings configure ordering dependencies between units. 

**before** ensures that the configured stack is executed before the
listed stacks, as the stack you are saying "I execute before these stacks".

**after** ensures the opposite, that the listed stacks are executed before
the configured stack, you are saying "I execute after these stacks".

For example, let's assume we have a project organized like this:

```
.
├── stack-a
│   └── terrastack.tsk.hcl
└── stack-b
    └── terrastack.tsk.hcl
```

And **stack-a/terrastack.tsk.hcl** looks like:

```
terrastack {
    required_version = "<version>"
}
```

And then we have **stack-b/terrastack.tsk.hcl**:

```
terrastack {
    required_version = "<version>"
}

stack {
    after = [
        "../stack-a"
    ]
}
```

This means that **stack-b** should be executed after **stack-a**.
For a scenario where both stacks have been detected as changed
the order of execution will be:

* stack-a
* stack-b

The same order of execution can be defined as:

**stack-a/terrastack.tsk.hcl**:

```
terrastack {
    required_version = "<version>"
}

stack {
    before = [
        "../stack-b"
    ]
}
```

**stack-b/terrastack.tsk.hcl**:

```
terrastack {
    required_version = "<version>"
}
```

This would also be a valid way to express the same order (although redundant):

**stack-a/terrastack.tsk.hcl**:

```
terrastack {
    required_version = "<version>"
}

stack {
    before = [
        "../stack-b"
    ]
}
```

**stack-b/terrastack.tsk.hcl**:

```
terrastack {
    required_version = "<version>"
}

stack {
    after = [
        "../stack-a"
    ]
}
```

You can also use **before** and **after** simultaneously on the same
stack for more complex scenarios. Lets say now we add a third **stack-c**
and the three stacks are defined as follows.

**stack-a/terrastack.tsk.hcl**:

```
terrastack {
    required_version = "<version>"
}
```

**stack-b/terrastack.tsk.hcl**:

```
terrastack {
    required_version = "<version>"
}
```

**stack-c/terrastack.tsk.hcl**:

```
terrastack {
    required_version = "<version>"
}

stack {
    before = [
        "../stack-b"
    ]
    after = [
        "../stack-a"
    ]
}
```

For a scenario where all 3 stacks have been detected as changed
the order of execution will be:

* stack-a
* stack-c
* stack-b


## Failure Modes

Currently the behavior when a stack execution fails given a total order of
execution is undefined. Whatever observed behavior should not be relied upon
since it may change on the future.


## What About Cycles ?

If any cycles are detected on the ordering definitions this will be
considered a failure and **terrastack** will abort with an
error message pointing out the detected cycle.
