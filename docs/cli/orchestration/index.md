---
title: Orchestration
description: Learn how to orchestrate the execution of commands or sequences of commands in stacks using the orchestration in Terramate.
---

# Orchestration

One of the most essential features of Terramate CLI is its ability to orchestrate the execution of commands in stacks, which allows to run commands such as `terraform apply` or `kubectl apply` across different stacks without having to navigate to each project stack individually.

Even in the case of environments where stacks are interdependent, Terramate’s orchestration ensures that operations are performed in the correct order, maintaining dependencies and preventing conflicts.

The orchestration engine provides various strategies for selecting stacks and configuring the execution order, which are explained in the following pages of this section.

## Default order of execution

In Terramate Projects, we can arrange stacks in a filesystem hierarchy. Parent stacks are always executed before their
child stacks in this arrangement. Thus, if stack A includes stack B, stack A will always be executed first.

This provides the ability to rearrange stacks, which can improve the mirroring of your cloud infrastructure and control
the sequence of execution, all without changing any code.

This ordering will fit well with the natural project organization and eliminate
the need for hard-coded dependencies between stacks.

Per default, commands such as [`run`](../cmdline/run.md) or [`list`](../cmdline/list.md) follow the default order of execution
so that parent stacks will run before child stacks.

The default order of execution can be altered in a stack's configuration. For details, please see
[configuring the order of execution](../stacks/configuration.md#configuring-the-order-of-execution).

::: tip
You can use the [list --run-order](../cmdline/list.md)
command to understand the order of execution of your stacks.
:::

## Ways to Run Commands

In Terramate, you have two main ways to run commands:

1. **Terramate Run Command:**
   Use this for running single, straightforward commands in your setup. It's great for quickly doing specific tasks across your infrastructure.

   Run a command in all stacks: 
   ```bash
   terramate run terraform init
   ```

2. **Terramate Scripts ([workflows](scripts.md)):**
   These are like playbooks for more complicated jobs. With Terramate Scripts, you can set up and run sequences of commands, automating complex processes. They're perfect for managing bigger tasks and making sure everything runs smoothly.

These two ways to orchestrate commands in Terramate give you everything you need to manage your commands, from simple tasks to bigger, more complicated workflows. Whether you're running one-off commands or managing a whole series of actions, Terramate has you covered.


## Change Detection Integration

Terramate offers seamless integration with [change detection](../change-detection/index.md), allowing users to optimize command orchestration by focusing only on the modified stacks. This section highlights how users can leverage the `terramate run` and `terramate script run` commands in conjunction with the `--changed` flag to facilitate this process.

By pairing the `--changed` flag with the `terramate run` command, users can instruct Terramate to execute commands exclusively on stacks that have undergone modifications since the last execution. Similarly, when using the `terramate script run` command, appending the `--changed` flag enables users to trigger the execution of Terramate Scripts solely on stacks with detected changes.

This feature makes it easier for users to handle resources effectively, especially when there are changes in the setup of their infrastructure.

## Sequential and Parallel Command Orchestration
Terramate CLI supports both sequential and parallel execution of commands to orchestrate stacks, thus accommodating both dependent and independent stacks. The CLI utilizes a [fork-join model](https://en.wikipedia.org/wiki/Fork%E2%80%93join_model) to execute the sequential parts (dependent stacks requiring specific order) and parallel parts (independent stacks that can be executed in any order). By leveraging this approach, Terramate ensures efficient execution while maintaining accuracy and consistency across deployments.

### Sequential
Stacks that have a dependency on other stacks need to be run sequentially.
Terramate cli runs [nested stacks](../stacks/nesting.md) in sequence as per the order of execution defined in the stack's configuration.

In Terramate, by default, commands are executed sequentially. When adhering to the default order of execution for a stack hierarchy as illustrated below:
```sh
/vpc
  /network_acl
  /internet_gateway
  /subnet
    /network_interface
    /ec2
  /route_table
  /security_group
```
When executed sequentially and respecting the nested layout, the execution sequence follows this pattern (in alphabetical order):

```sh
terramate list --run-order

vpc
vpc/internet_gateway
vpc/network_acl
vpc/route_table
vpc/security_group
vpc/subnet
vpc/subnet/ec2
vpc
```
As an illustration, to execute a command sequentially across all stacks within a particular directory:
```bash
terramate run --chdir stacks/vpc -- terraform init
```
### Parallel

Terramate facilitates parallel execution, enabling independent stacks to run in parallel, thereby offering significant time savings, particularly during commands like `terraform init`. Despite the parallel nature of execution, Terramate ensures that the order of execution is still respected.

This approach notably diminishes build time consumption for deployments and drift detection to a bare minimum, while also reducing waiting time for users when executing commands across stacks. Moreover, with Terramate Cloud, users can conveniently access logs of all executed stacks in the correct order, further enhancing visibility and monitoring capabilities during the execution process.

To initiate parallel execution, users can utilize the `--parallel N` flag, where N represents the number of parallel processes desired. This allows users to tailor the level of parallelism according to their specific requirements.
For example:
```bash
terramate run --parallel=5 terraform init
```
This command runs `terraform init` in parallel across all stacks while maintaining the specified order.