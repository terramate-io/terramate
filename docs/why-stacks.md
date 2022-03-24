# Why Use Stacks ?

The stack concept is advised for several reasons. But before we go into
**why** lets talk a little about **what** a stack is.

The stack concept is not defined by Hashicorp's Terraform tooling but just a
convention used by the _Terraform community_, so a stack can be loosely defined as:

```
A terraform stack is a runnable terraform module that operates on a subset of
the infrastructure's resources.
```

By _runnable_ it means it has everything needed to call
`terraform init/plan/apply` . For example, a module used by other modules, ie,
don't have the `provider` explicitly set, is not runnable hence it's
**not a stack**.

If your runnable terraform module creates your whole infrastructure, *it's
also not a stack*, since the idea of stacks is to be a unit of infrastructure
decomposition.

## Isolate code with higher change frequency

If you infrastructure have a high frequency of change, for example, several
deploys per day/week or several configuration changes per day/week, then if you
apply the change in your single Terraform project, some dependent resources can
be recreated leading to increased downtime. There are several reasons why a
dependent module can be recreated, eg.: attributes with variable interpolation
from recreated resources; underspecified attributes refreshing during plan, etc.

By using stacks you can modify only the stacks affected by the deploys or
configuration changes needed, but you have to choose the size of the stack
wisely to avoid duplication.

The overall principle being, code that changes at different rate change for
different reasons and code that changes for different reasons should be as
isolated from each other as possible.

## Reduce the blast radius

A small change can lead to catastrophic events if you're not careful or makes a
mistake like forgetting a "prevent_destroy" attribute in the production database
lifecycle declaration. Someone may commit mistakes and it's better if
you could reduce the impact of such errors.
An extreme example is: avoiding a database instance destroy because of a dns TTL
change.

## Reduce execution time

By using stacks you can reduce the time a infrastructure change takes to finish.
This is even more appealing if your Terraform changes are applied through CI
builders running in the cloud because faster integration/builds leads to reduced
cloud costs.

## Ownership

In the case of a big organization you probably don't want a single person or
team responsible for the whole infrastructure. The company's stacks can be
spread over several repositories and managed by different teams.

By having this separation, it also makes it easier when you want separation
by resource permissions, so having some stacks that can only be run by
specific individuals or teams.

## Others

There are lots of other opinionated reasons why stacks would be a good option:
stacks per environment or deploy region location, stacks per topic (IAM vs
actual resources) and so on.
