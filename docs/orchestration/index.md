---
title: Stacks Execution Orchestration | Terramate
description: Terramate adds powerful capabilities such as code generation, stacks, orchestration, change detection, data sharing and more to Terraform.

prev:
  text: 'Stack Configuration'
  link: '/stacks/'

next:
  text: 'Tag Filter'
  link: '/tag-filter'
---

# Stacks Execution Orchestration

The orchestration of stack's execution is driven by the `terramate run` command
and it supports different ways of selecting stacks and configuring the order of
execution.

## Stacks selection

Choosing the right stacks for execution is a key part of Stacks Execution Orchestration. Terramate offers three ways to select stacks:

1. Change detection

The [change detection](../change-detection/index.md) selects only the stacks that have been modified since the last execution.

2. Current directory

Terramate uses the current directory it is being executed to filter out stacks, ie., limit the scope
of the execution, so if you execute `terramate` from the project's root
directory, all stacks will be selected, and changing to inner directories in the
project structure will select only stacks that are children of the current directory.

3. Explicit `wants` relationship.

 The `wants` attribute of the stack block defines an explicit relationship between a stack and its list of wanted stacks. When a stack is selected, all the stacks listed on its `wants` list will also be selected, independent of any other selection criteria.

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

Two methods can help us manage this issue in Terramate - the Filesystem Hierarchical Order and the Explicit Order of Execution.

### Filesystem hierarchical order

Within Terramate, we can arrange stacks in a filesystem hierarchy. Parent stacks are always executed before their child stacks in this arrangement. Thus, if **stack A** includes **stack B**, **stack A** will always be executed first. 

Keep in mind, incorrect stack order might lead Terramate to attempt resource creation before its dependencies are ready, causing errors. So, a clear stack order is necessary to avoid such issues.

### Explicit Order Of Execution

Terramate's Explicit Order of Execution feature allows you to designate a specific order for stack execution. This is done by definin **before** and **after** fields within the **stack** block, referring to other directories containing stacks.

**Before** ensures that the specified stack is executed before all stacks in the mentioned directories. Conversely, **after** ensures the stack is executed after those in the directories. Both relative and project root paths can be used to reference directories. 

Note, a parent stack can never be executed after a child stack, as this would cause a cycle error.

Here's how to use this feature:

1. Open your Terramate configuration file `terramate.tm.hcl` in a text editor.
2. Select the stack block you want to allocate a specific order.
3. Depending on your needs, add either the **before** or **after** field inside the stack block.
4. Add **string** values representing the directory paths you want to order execution for in the **before** or **after** field.
5. Save and close the file.

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

In both configurations, the execution sequence will be **stack-a** -> **stack-b**

You can also use the **before** field to define the execution order. For instance:

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

The above configuration also results in the execution sequence **stack-a** -> **stack-b**.

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

With these settings, the execution sequence will be **stack-a** -> **stack-c** -> **stack-b**.

One command that considers this execution order is `terramate run`, which runs `terraform plan` on all stacks in the defined sequence:

```sh
terramate run terraform plan
```

## Failure Modes

The current behavior during complete failure in stack order execution remains undefined. 
As this behavior is subject to change, it is advisable not to rely on it.

### Handling Cycles Conflicts

Cyclic dependency occurs when circular dependencies exist between the stacks. For instance, if stack A is dependent on stack B, and vice versa, a cycle is created. 

This cycle can lead to execution failure if not properly configured. When such an event occurs, Terramate will terminate the process, providing an error message indicating the detected cycle.

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

In this situation, a conflict arises causing the execution to enter failure mode. An error message will be reported in such an instance.
