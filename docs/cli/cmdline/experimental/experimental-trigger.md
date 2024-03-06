---
title: terramate experimental trigger - Command
description: Mark a stacks as changed so they will be triggered in Change Detection by using the `terramate experimental trigger` command.
---

# Trigger

::: warning
This is an experimental command and is likely subject to change in the future.
:::

The `terramate experimental trigger` command forcibly marks a stack as "changed" even if it doesn't contain any code changes according to the
[change detection](../../change-detection/index.md). It does this by creating a file (by default in `/.tmtriggers`)
which should then be committed. `terramate run` will then execute commands against any stacks that have been triggered
in the last commit (as well as any other changed stacks).

The trigger mechanism has various use cases. It may be that a previous CI/CD run failed or that you have detected a drift between the code and the actual state. For those using Terramate Cloud, the additional `--cloud-status=<status>` argument can be used to trigger stacks that are in one of the following states:

| Status      | Meaning                                                                  |
| ----------- | ------------------------------------------------------------------------ |
| `ok`        | The stack is not drifted and the last deployment succeeded               |
| `failed`    | The last deployment of the stack failed so the status is unknown         |
| `drifted`   | The actual state is different from that defined in the code of the stack |
| `unhealthy` | This meta state matches any undesirable state (failed, drifted etc)      |
| `healthy`   | This meta state matches stacks that have no undesireable state           |

## Usage

`terramate experimental trigger PATH`

## Examples

Create a change trigger for a stack:

```bash
terramate experimental trigger /path/to/stack
```

Create triggers for all stacks that have drifted

```bash
terramate experimental trigger --cloud-status=drifted
```
