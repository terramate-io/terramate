---
title: Stacks Execution Orchestration
description: Learn how to orchestrate the execution of commands in stacks with the terramate run command.
prev:
  text: 'Stack Configuration'
  link: '/stacks/'

next:
  text: 'Tag Filter'
  link: '/tag-filter'
---

# Stacks Execution Orchestration

The orchestration of stack's execution is driven by the [terramate run](../cmdline/run.md) command,
and it supports different ways of selecting stacks and configuring the order of
execution.

## Stacks selection

The **selection** defines which stacks from the whole set must be selected to execute and terramate provides three ways of configuring that:

1. Change detection

The [change detection](../change-detection/index.md) filter out stacks not changed.

2. Current directory

Terramate uses the current directory it is being executed to filter out stacks, ie., limit the scope
of the execution, so if you execute `terramate` from the project's root
directory, all stacks will be selected, and changing to inner directories in the
project structure will select only stacks that are children of the current directory.

3. Explicit `wants` relationship.

The `wants` attribute of the stack block defines an explicit relationship between a stack and its list of wanted stacks.
When a stack is selected, all the stacks listed on its `wants` list will also be selected, independent of any other selection criteria.

Example:

```hcl
stack {
    wants = [
        "/other/stack1",
        "/other/stack2"
    ]
}
```

These 3 selection methods could be used together, and the order which they are
applied is: `change detection`, `current directory`, `wants`.

## Stacks ordering

Stacks in deployment can either be independent or dependent on one another. In certain scenarios, the output from one stack may be required as input for another. In these cases, the execution order is crucial to ensure that all dependencies are met and resources are available when needed.

To illustrate, consider two stacks - **stack A** and **stack B**. If **stack A** creates a database and **stack B** sets up a web server dependent on that database, the execution order matters. If **stack B** is deployed before **stack A**, the web server won't connect to the non-existent database.

This can be done through data sources or
by [loading the state](https://www.terraform.io/docs/language/state/remote-state-data.html)
of another stack, or even an implicit dependency like hard coding the name/ID.

Two methods can help us manage this issue in Terramate - the [Filesystem Hierarchical Order](https://terramate.io/docs/cli/orchestration/#filesystem-hierarchical-order) and the [Explicit Order of Execution](https://terramate.io/docs/cli/orchestration/#explicit-order-of-execution).

### Filesystem hierarchical order

Within Terramate, we can arrange stacks in a filesystem hierarchy. Parent stacks are always executed before their child stacks in this arrangement. Thus, if **stack A** includes **stack B**, **stack A** will always be executed first.

This process carries both benefits and risks. On the positive side, it offers you the ability to rearrange stacks, which can improve the mirroring of your cloud infrastructure and control the sequence of execution, all without changing any code. On the flip side, you must exercise caution. Rearranging stacks merely for aesthetic appeal might unintentionally change the execution sequence, which could lead to operational problems.


### Explicit Order Of Execution

This feature allows you to designate a specific order for stack execution. This is done by defining **before** and **after** fields within the **stack** block, referring to other directories containing stacks.

**Before** ensures that the specified stack is executed before all stacks in the mentioned directories. Conversely, **after** ensures the stack is executed after those in the directories. Both relative and project root paths can be used to reference directories.

> **Note:** A parent stack can never be executed after a child stack, as this would cause a cycle error.

This can be configured by adding `before` and `after` fields to the stack block.

```hcl
stack {
    before = [
        "/other/stack"
    ]
}
```

For example, let's assume we have a project organized like this:

```
.
├── stack-a
│   └── terramate.tm.hcl
└── stack-b
    └── terramate.tm.hcl
```

In this case, **stack-a/terramate.tm.hcl** would look like this:


```hcl
stack {}
```


And then we have **stack-b/terramate.tm.hcl**:


```hcl
stack {
    after = [
        "../stack-a"
    ]
}
```

Alternatively, you can define the same using a project root relative path:


```hcl
stack {
    after = [
        "/stack-a"
    ]
}
```

In both configurations, the execution sequence will be:

* stack-a
* stack-b

The same order of execution can be defined using **before**. For instance:

In `stack-a/terramate.tm.hcl`:

```hcl
stack {
    before = [
        "../stack-b"
    ]
}
```

And `stack-b/terramate.tm.hcl`:

```hcl
stack {}
```

The above configuration also results in the execution sequence:

* stack-a
* stack-b

For more complex scenarios, you can use both **before** and **after** fields in the same stack block. For example, let's add a third **stack-c** to our project:

**stack-a/terramate.tm.hcl**:

```hcl
stack {}
```

**stack-b/terramate.tm.hcl**:

```hcl
stack {}
```

**stack-c/terramate.tm.hcl**:

```hcl
stack {
    before = [
        "../stack-b"
    ]
    after = [
        "../stack-a"
    ]
}
```

With these settings, the execution sequence will be:

* stack-a
* stack-c
* stack-b

One command that considers this execution order is `terramate run terraform plan`, which runs on all stacks in the defined sequence:

```sh
terramate run terraform plan
```

### Change Detection And Ordering

When using any terramate command with support to change detection,
execution order is only imposed on stacks detected as changed. If a stack
is mentioned on **before**/**after** but the mentioned stack has no changes
on it, it will be ignored when calculating order.

An example of such a command would be using terramate to run **terraform apply**,
but only on changes stacks, like this:

```bash
terramate run --changed terraform apply
```

The overall algorithm for this case:

* Check which stacks have changed, lets call the result a **changeset**
* Ordering is established on top of the previously calculated **changeset**

Given that we have 3 stacks, **stack-a**, **stack-b**, **stack-c**.
**stack-a** has no ordering requisites.
**stack-b** defines this order:

```hcl
stack {
    after = [
        "../stack-a",
    ]
}
```

**stack-c** defines this order:

```hcl
stack {
    after = [
        "../stack-a",
        "../stack-b",
    ]
}
```

The **static** order is defined as:

* stack-a
* stack-b
* stack-c

If the **changeset=('stack-a', 'stack-c')**, this will be the **runtime** order:

* stack-a
* stack-c

Even though **stack-c** defined that it needs to be run after **stack-b**, since
**stack-b** has no changes on it, it will be ignored when defining the
**runtime** order.


## Stack Execution Environment

It is possible to control the environment variables of commands when they are
executed on a stack. That is done through the `terramate.config.run.env` block.
More details on how to use can be find [Project Configuration](../configuration/project-config.md#terramateconfigrunenv)
documentation.


## Failure Modes

The current behavior during complete failure in stack order execution remains undefined. As this behavior is subject to change, it is advisable not to rely on it.


### Handling Cycles Conflicts

Whenever Terramate finds any repeated patterns or **cycles** in the way the resources are organized or defined, it will interpret this as a error. When this happens, Terramate will abort the process and display a fatal error message. This error message will indicate where the repeated pattern was identified.

Consider a stack defined as follows in **stack-a/terramate.tm.hcl**:

```hcl
stack {
    before = [
        "../stack-b"
    ]
    after = [
        "../stack-b"
    ]
}
```

In this situation, a conflict arises causing the execution to enter failure mode, then a fatal error message will be reported.
