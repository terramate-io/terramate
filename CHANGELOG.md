# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## Versioning

This Solution adheres to the principles of [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

Given a version number `MAJOR.MINOR.PATCH`, we increment the:

1. `MAJOR` version when we make incompatible changes,
2. `MINOR` version when we add functionality in a backward compatible manner, and
3. `PATCH` version when we make backward compatible bug fixes.

### Backward compatibility in `0.0.z` and `0.y.z` version

- Backward compatibility in versions `0.0.z` is **not guaranteed** when `z` is increased.
- Backward compatibility in versions `0.y.z` is **not guaranteed** when `y` is increased.

## Unreleased

### Added

- Terragrunt change detection.
  - Terramate understands the structure of `terragrunt.hcl` files and detects changes if any other Terragrunt referenced file changes (`dependency`, `dependencies` and `include` blocks, function calls like `find_in_parent_folders()`, `read_terragrunt_config()`, etc).
- Automatically add stack ordering to Terragrunt stacks created by `terramate create --all-terragrunt`.

## v0.5.0

### BREAKING CHANGES

> [!IMPORTANT]
> When using nested stacks and tags in `before` and `after` the order of execution was wrong.
> This is now fixed but but can lead to a change in the order of execution in some rare cases.
> Please check the `terramate list --run-order` after upgrading to ensure you run stacks in the correct order.

### Added

- Add `terramate.config.generate.hcl_magic_header_comment_style` option to change the comment style for magic headers to `#` instead of `//`
- Add support for formatting single files or stdin with `terramate fmt`
- Add support for `--cloud-status` filter to `terramate run`
- Add support for `--cloud-status` filter to `terramate script run`
- Add support to synchronize previews to Terramate Cloud via new `terramate run --cloud-sync-preview`
- Add `script.name` attribute. 
  - The commands `terramate script info`, `terramate script list` and `terramate script tree` were updated to show the script name (when available).
- Improve user experience when using Terramate with existing Terragrunt projects.
  - Add  `terramate create --all-terragrunt` option, which will automatically create Terramate stacks for each Terraform module.
- Allow to run independent stacks in parallel for faster deployments and better utilization of system resources in general.
  - Add `--parallel=N` (short `-j N`) option to `terramate run` and `terramate script run` to allow running up to `N` stacks in parallel.
  - Ordering constraints between stacks are still respected, i.e. `before`/`after`, parent before sub-folders.
- Add `cloud_sync_drift_status` option to `script` block commands. It allows for synchronizing the
  stack drift details from script jobs.
- Add --cloud-sync-layer to allow users to specify a preview layer, e.g.: `stg`, `prod` etc.
  - This is useful when users want to preview changes in a specific terraform workspace.
- Add `--cloud-sync-layer` and `--cloud-sync-preview` to `script` block, this would allow users to synchronize previews to Terramate Cloud via script jobs.

### Fixed

- Fix a panic in language server with a project containing errors on root directory
- Fix the execution order when using `tag` filter in `after/before` in conjunction with implicit order for nested stacks. (BREAKING CHANGE)
- Fix escape sequences being interpreted in heredocs (issue #1449)

## v0.4.6

### Fixed

- Use `repository` filter when listing Terramate Cloud stacks.
  - It makes the `--cloud-status=<status>` flag faster and potentially less brittle for cases where other repositories have issues.

## 0.4.5

### Added

- Add support for `stack_filter` blocks in `generate_file` blocks
- Add `list --run-order` flag to list stacks in the order of execution
- Add support for `terramate` in linting, pre-commit and test environments

  - Add `--detailed-exit-code` to `terramate fmt` and `terramate generate` commands:

    - An exit code of `0` represents successful execution and no changes made
    - An exit code of `1` represents the error state, something went wrong
    - An exit code of `2` represents successful execution but changes were made

### Changed

- Refactor Safeguard

  - Add `disable_safeguards` configuration and `--disable-safeguards` CLI option with possible values
    - `all` Disable ALL safeguards (use with care)
    - `none` Enable ALL safeguards
    - `git` Disable all git related safeguards:
      - `git-untracked` Disable Safeguard that checks for untracked files
      - `git-uncommitted` Disable Safeguard that checks for uncomitted files
      - `git-out-of-sync` Disable Safeguard that checks for being in sync with remote git
    - `outdated-code` Disable Safeguard that checks for outdated code

- Promote cloud commands

  - `terramate cloud login`
  - `terramate cloud info`
  - `terramate cloud drift show`

- Improve support for synchronization of deployments to Terramate Cloud

  - Add `cloud_sync_deployment` flag to Terramate Scripts Commands
  - Add `cloud_sync_terraform_plan_file` flag to Terramate Scripts Commands when synchronizing deployments.
  - Add `--cloud-sync-terraform-plan-file` support to `terramate run` when synchronizing deployments.

- Promote `--experimental-status` flag to `--cloud-status` flag in

  - `terramate experimental trigger`
  - `terramate list`

### Fixed

- Fix a performance issue in `tm_dynamic.attributes` configuration
- Fix order of execution in `terramate script run`
- Fix a type issue when assigning lists to `script.job.command[s]`

### Deprecated

- Old safeguard configuration options are now considered deprecated and will issue a warning when used in upcoming releases.

## 0.4.4

### Added

- Add `terramate.config.experiments` configuration to enable experimental features
- Add support for statuses `ok`, `failed`, `drifted`, and `healthy` to the `--experimental-status` flag
- Add experimental `script` configuration block

  - Add `terramate script list` to list scripts visible in current directory
  - Add `terramate script tree` to show a tree view of scripts visible in current directory
  - Add `terramate script info <scriptname>` to show details about a script
  - Add `terramate script run <scriptname>` to run a script in all relevant stacks

- Add `stack_filter` block to `generate_hcl` for path-based conditional generation.

- Promote experimental commands

  - `terramate debug show metadata`
  - `terramate debug show globals`
  - `terramate debug show generate-origins`
  - `terramate debug show runtime-env`

- Improvements in the output of `list`, `run` and `create` commands.

### Fixed

- Fix an issue where `generate_file` blocks with `context=root` were ignored when defined in stacks
- Fix `experimental eval/partial-eval/get-config-value` to not interpret the output as a formatter
- Fix an issue where change detector cannot read global/user git config

## 0.4.3

### Added

- Add `--cloud-sync-terraform-plan-file` flag for synchronizing the plan
  file in rendered ASCII and JSON (sensitive information removed).
- Add configuration attribute `terramate.config.cloud.organization` to select which cloud organization to use when syncing with Terramate Cloud.
- Add sync of logs to _Terramate Cloud_ when using `--cloud-sync-deployment` flag.
- Add `terramate experimental cloud drift show` for retrieving drift details from Terramate Cloud.
- Add support for cloning nested stacks to `terramate experimental clone`. It can also be used to clone directories that
  are not stacks themselves, but contain stacks in sub-directories.

### Changed

- Improve diagnostic messages on various errors

## 0.4.2

### Added

- Add `--cloud-sync-drift-status` flag for syncing the status of drift detection
  to the Terramate Cloud.

### Fixed

- Ensured that SIGINT aborts execution of subsequent stacks in all cases.
- Removed non-supported functions (`tm_sensitive` and `tm_nonsensitive`)

## 0.4.1

### Added

- Add support for globs in the `import.source` attribute to import multiple files at once.
- Add support for listing `unhealthy` stacks with `terramate list --experimental-status=unhealthy`.
- Add support for triggering `unhealthy` stacks with `terramate experimental  trigger --experimental-status=unhealthy`.
- Add support for evaluating `terramate run` arguments with the `--eval` flag.

### Fixed

- Allow to specify multiple tags separated by comma when using `terramate create --tags` command.
- Fixed inconsistent behaviour in `terramate create` vs. `terramate create --all-terraform`, both now populate the name/description fields the same way.

## 0.4.0

### Added

- Introduce the `terramate create --all-terraform` command that can be used to initialize plain terraform projects easily with a Terramate Stack configuration.
  Every directory containing Terraform Files that configure either a `terraform.backend {}` block or a `provider {}` block will be initialized.
- Introduce `terramate create --ensure-stack-ids` to add a UUID as stack ID where `stack.id` is not set. (Promoted from `experimental ensure-stack-id`)

### Changed

- BREAKING CHANGE: Make `stack.id` case insensitive. If you used case to identify different stacks, the stack ID need to be adjusted. If you use the recommended UUIDs nothing has to be done.
- We now distribute the Terramate Language Server `terramate-ls` alongside `terramate` to be able to pick up changes faster for IDEs.

## 0.3.1

### Added

- Introduce the `terramate experimental cloud info` command.
- Introduce the `--cloud-sync-deployment` flag (experimental).
- Introduce the `terramate experimental ensure-stack-id` command.

### Fixed

- Fix a crash when `generate_hcl {}` has no `content {}` block defined.

## 0.3.0

### Added

- Introduced the `terramate experimental cloud login` command.

### Fixed

- Fix a file descriptor leak when loading the configuration tree.
