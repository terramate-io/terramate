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

## Defining Dependency

## Order of Execution

## What About Cycles ?
