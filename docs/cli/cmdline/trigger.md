---
title: terramate trigger - Command
description: With the terramate trigger command you can mark a stack to be considered by the change detection.

prev:
  text: 'Run'
  link: '/cli/cmdline/run'

next:
  text: 'Vendor Download'
  link: '/cli/cmdline/vendor-download'
---

# Trigger

**Note:** This is an experimental command that is likely subject to change in the future.

The `trigger` command forcibly marks a stack as "changed" (see [change detection](../change-detection/index.md)). It does this by creating a file (by default in `/.tmtriggers`) which should then be committed. `terramate run` will then execute commands against any stacks that have been triggered in the last commit (as well as any other changed stacks).

The trigger mechanism has various use cases. It may be that a previous CI/CD run failed or that you have detected drift between the code and the actual state. For those using Terramate Cloud, the additional `--experimental-status=<status>` argument can be used to trigger stacks that are in one of the following states:

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
terramate experimental trigger --experimental-status=drifted
```
