# Stacks and Stack Status Visibility

The Stacks list helps you to keep an overview of all stacks managed with Terramate.

::: tip
[Stacks](../../cli/stacks/index.md) combine infrastructure code, state *(often managed in a remote backend)*, and configuration
to deploy and manage cloud infrastructure as isolated units.
:::

![Stacks Overview](../assets/stacks.png "Terramate Cloud Stacks Overview")

That can be created and configured with [Terramate CLI](../../cli/stacks/create.md), located in a `repository` and synced
to Terramate Cloud.

As the stack within a `repository` is only plain code and configuration, Terramate Cloud offers to synchronize a state of
all stacks when orchestrated with Terramate CLI and keep track of this state over multiple deployments or drift runs.

The state of a stack is not to be confused with a Terraform state defined by the Terraform Backend Configuration.
Still, the Terramate Cloud State of a stack includes this information and extends it with multiple status values and metadata.

In addition, stack visibility is not limited to single stacks or single repositories but combines all stacks in all your 
organization's repositories in a central place.

Each stack can be `healthy` or `unhealthy` (e.g. failed or drifted) depending on the result of deployments or drift runs.
