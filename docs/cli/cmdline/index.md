---
title: Terramate CLI
description: Manage Stacks, Run Commands, Generate Code, Plan & Deploy IaC and Synchronize to and from Terramate Cloud to keep our IaC healthy.
---

# Command Line Interface (CLI)

Terramate CLI helps you manage your daily IaC workloads:

- Manage stacks, maintain your IaC Code base, and keep it DRY via code generation.
- Plan & Deploy your IaC Changes in an effective way using advanced orchestration and change detection
- Synchronize your distributed IaC set ups to and from Terramate Cloud to improve observability and keep your code base heatlhy

This section will provide information to the most used CLI commands and also offers a complete reference of CLI commands available.

## Quickstart

Please check out our [quick start guide](../getting-started/index.md) or continue reading to see a summary of the commands used daily when working with terramate.

## Create a Stack

```bash
terramate create --name "My New Stack" path/to/stack
```

## Import Terraform configurations

Import existing Terragrunt projects in no time.

```bash
terramate create --all-terraform
```

## Import Terragrunt configurations

Import existing Terragrunt projects in no time.

```bash
terramate create --all-terragrunt
```

## List changed stacks

List stacks changed in the current branch or since the last merged pull request

```bash
terramate list --changed
```

## Run commands in stacks

Terramate can execute any command in stack.
Commands can be executed in parallel to speed up the execution.

```bash
terramate run --parallel 20 -- terraform init
terramate run -- terraform plan
```

## Run Terramate Script in stacks

A Terramate Script provides a simple interface (`terraform deploy`) for running a complex sequence of commands like `terrform init`, `terraform validate`, `terraform plan`, `terraform apply`.
Scripts are user defined and can execute any command.

```bash
terramate script run --changed terraform deploy
```

## Trigger Code Generation

```bash
terramate generate
```

## Login to Terramate Cloud

Bring Terramate CLI to the next level by enhancing it with Terramate Cloud Features.
After Sign Up you can login from CLI to start synchronizing data to the Cloud.
This is mostly useful in automation but can also be used from your local machine.
Many command support cloud features.

```bash
terramate cloud login
```

## Reconcile detected drifts

Combining Terramate CLI and Cloud can support you to reconcile known drifts:
The following command runs a `terraform deploy` Terramate Script (to be defined by you) on all drifted stacks (as known by Terramate Cloud) on all stacks that are tagged with `auto-reconcile-drift`.
You can add this to your CI/CD automation to auto reconcile drifts when they happen.

```bash
terramate script run --cloud-status drifted --tags auto-reconcile-drift terraform deploy
```

## Available global options

To view a list of available commands and global flags, execute `terramate --help`.
All sub-commands support the `--help` flag as well for specific details.

### Options

- `-h, --help` Show context-sensitive help.
- `-C, --chdir <directory>` Sets working directory.
- `-v, --verbose <level>` Increase verboseness of output.
- `--quiet` Disable output.

- `--log-level <level>` Log level to use: 'disabled', 'trace', 'debug', 'info', 'warn', 'error', or 'fatal'
- `--log-fmt <format>` Log format to use: 'console', 'text', or 'json'.
- `--log-destination <channel>` Destination of log messages.

<!-- - `--disable-check-git-untracked`      Disable git check for untracked files. -->
<!-- - `--disable-check-git-uncommitted`    Disable git check for uncommitted files. -->

## Auto Completions

Terramate supports autocompletion of commands for `bash`, `zsh` and `fish`. To
install the completion just run the following command:

```bash
terramate install-completions
```

## CLI Configuration

Terramate supports a per-user configuration file called `.terramaterc` (or
`terramate.rc` on Windows) which applies to all Terramate projects in the user's
machine by setting up default values for some CLI flags.

The configuration file is a simple HCL file containing top-level attributes.
The attribute expressions must only contain literal values (`number`, `string`,
etc) and no function calls.

Not all CLI flags can be configured by the configuration file.

Below is a list of options:

- `user_terramate_dir` (`string`)

Configures an alternative location for the local `~/.terramate.d` (or `%APPDATA%/.terramate.d`
on Windows).

- `disable_checkpoint` (`boolean`)

When set to `true`, disables [upgrade and security bulletin checks](../configuration/upgrade-check.md). This is similar to exporting the `DISABLE_CHECKPOINT=1` environment variable.

- `disable_checkpoint_signature` (`boolean`)

when set to `true`, still allows the [upgrade and security bulletin checks](../configuration/upgrade-check.md)
described above but disables the use of an anonymous id used to de-duplicate warning messages.

### Location

The configuration should be placed in a different path depending on the operating
system:

On _Windows_, the file must be named `terraform.rc` and placed in the user's
`%APPDATA%` directory. The location of this directory depends on your _Windows_
version and system configuration. You can use the command below in _PowerShell_ to
find its location:

```PowerShell
$env:APPDATA
```

On Unix-based systems (_Linux_, _MacOS_, _\*BSD_, etc), the file must be named
`.terraformrc` and placed in the home directory of the relevant user.

The default location of the Terramate CLI configuration file can also be specified
using the `TM_CLI_CONFIG_FILE` environment variable.
Example:

```bash
TM_CLI_CONFIG_FILE=/etc/terramaterc terramate run -- <cmd>
```
