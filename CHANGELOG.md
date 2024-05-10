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

## v0.8.3

### Fixed

- Fix potential crash when trying to obtain the pull request metadata associated with a Github repository.

## v0.8.2

### Added

- Add support for Gitlab `id_token` OIDC.
  - You can connect to Terramate Cloud using [Gitlab id_token](https://docs.gitlab.com/ee/ci/secrets/id_token_authentication.html) exported as a `TM_GITLAB_ID_TOKEN` environment variable.

### Fixed

- Fix issue with handling paginated responses from Github API when retrieving review and GHA action metadata.

## v0.8.1

### Fixed

- Fix incorrect warning emitted by the parser in the case `terramate.config.run.env` is defined outside project root.

## v0.8.0

### Added

- Add support for hierarchical configuration of the stack run-time environment variables.
  - The `terramate.config.run.env` declared closer to the stack has precedence over
    declarations in parent directories.
- Full support for OpenTofu plan files when synchronizing deployments, drifts or previews to Terramate Cloud:
  - Add CLI option `--tofu-plan-file=FILE`
  - Add Terramate Scripts command option `tofu_plan_file = FILE`

### Changed

- (**BREAKING CHANGE**) Use `terramate.required_version` to detect project root if git is absent.
  - This is only a breaking change for projects not using `git`.

## (v0.7.0)

### Tested

- Issue a test-only release that includes everything that was published in v0.6.5

## v0.6.5

### Added

- Add `terramate.config.change_detection.terragrunt.enabled` attribute. It supports the values below:
  - `auto` (_default_): Automatically detects if Terragrunt is being used and enables change detection if needed.
  - `force`: Enables Terragrunt change detection even if no Terragrunt file is detected in any stack.
  - `off`: Disables Terragrunt change detection.

### Fixed

- Fix a performance regression in repositories having a lot of stacks.

## v0.6.4

### Fixed

- Fix `terramate fmt --detailed-exit-code` not saving the modified files.

## v0.6.3

### Fixed

- Fix the `generate_*.inherit=false` case when the blocks are imported and inherited in child stacks.

## v0.6.2

### Fixed

- Fix remaining naming inconsistencies of some `cloud` script options.
  - Rename CLI option `--sync-layer` to `--layer`
  - Rename CLI option `--sync-terraform-plan-file` to `--terraform-plan-file`
  - Rename Terramate Script option `sync_layer` to `layer`
  - Rename Terramate Script option `sync_terraform_plan_file` to `terraform_plan_file`
  - Old flags and command options are still supported as aliases for the new ones.

## v0.6.1

### Experiments

- Promote Terragrunt integration experiment to stable.
  - Please remove `terragrunt` from `terramate.config.experimental` config.
- Add new experiment `tmgen` which allows to generate HCL files in stacks without `generate_hcl` configurations.
  - Please enable the experiment with `terramate.config.experimental = ["tmgen"]`.

### Added

- Add `generate_*.inherit` attribute for controlling if generate blocks must be inherited
  into child stacks.
- Add `terramate cloud login --github` for authenticating with the GitHub account.
- Add `terramate create --watch` flag for populating the `stack.watch` field at stack creation.
- Add support for parsing and generating code containing HCL namespaced functions.
  - Check [HCL Changes](https://github.com/hashicorp/hcl/pull/639) for details.

### Changed

- Rename Terramate Cloud options
  - Rename CLI option `--cloud-sync-deployment` to `--sync-deployment`
  - Rename CLI option `--cloud-sync-drift-status` to `--sync-drift-status`
  - Rename CLI option `--cloud-sync-preview` to `--sync-preview`
  - Rename CLI option `--cloud-sync-layer` to `--sync-layer` (WARN: fixed in next release to `--layer`)
  - Rename CLI option `--cloud-sync-terraform-plan-file` to `--sync-terraform-plan-file` (WARN: fixed in next release to `--terraform-plan-file`)
  - Rename CLI option `--cloud-status` to `--status`
  - Rename Terramate Script option `cloud_sync_deployment` to `sync_deployment`
  - Rename Terramate Script option `cloud_sync_drift-status` to `sync_drift_status`
  - Rename Terramate Script option `cloud_sync_preview` to `sync_preview`
  - Rename Terramate Script option `cloud_sync_layer` to `sync_layer` (WARN: fixed in next release to `layer`)
  - Rename Terramate Script option `cloud_sync_terraform_plan_file` to `sync_terraform_plan_file` (WARN: fixed in next release to `terraform_plan_file`)
  - Rename Terramate Script option `cloud_status` to `status`
  - Old flags and command options are still supported as aliases for the new ones.

### Fixed

- Fix a panic in the language server when editing a file outside a repository.

## v0.6.0

### Added

- (**BREAKING CHANGE**) Enable change detection for dotfiles. You can still use `.gitignore` to ignore them (if needed).
- Add a new flag `--continue-on-error` to `terramate script run`. When the flag
  is set and a command in a script returns an error:
  - The script execution will be aborted and no further commands or jobs from that script will be run on the current stack node.
  - The script execution will continue to run on the next stack node.
  - `terramate script run` will return exit code 1 (same behavior as `terramate run --continue-on-error`).
- Add a new flag `--reverse` to `terramate script run`. When the flag is set, the script execution will happen in the reverse order of the selected stacks. This is similar to `terramate run --reverse`.
- Improve Terragrunt Integration for Terramate Cloud by adding `--terragrunt` flag and `terragrunt` script option to use `terragrunt` binary to create change details when synchronizing to Terramate Cloud

### Fixed

- Fix a bug in the dotfiles handling in the code generation. Now it's possible to generate files such as `.tflint.hcl`.
- Fix the cloning of stacks containing `import` blocks.

### Changed

- (**BREAKING CHANGE**) Removes the option `terramate.config.git.default_branch_base_ref`.
- (**BREAKING CHANGE**) The code generation of HCL and plain files was disallowed inside dot directories.

## v0.5.5

### Added

- Add `script.job.name` and `script.job.description` attributes.

## v0.5.4

### Fixed

- Fix `--cloud-status` flag when stacks were synchronized with uppercase in the `stack.id`.
- Fix `terramate cloud drift show` when used in stacks containing uppercase in the `stack.id`.
- Fix `cloud_sync_terraform_plan_file` option for previews in scripts

## v0.5.3

### Fixed

- Fix an issue that prevented stack previews from being created when using uppercase letters in stack IDs.

## v0.5.2

### Fixed

- Fix an inconsistency in `stack.id` case-sensitivity when `--cloud-sync-*` flags are used.

## v0.5.1

### Added

- Add Terragrunt change detection.
  - Terramate understands the structure of `terragrunt.hcl` files and detects changes if any other Terragrunt referenced file changes (`dependency`, `dependencies` and `include` blocks, function calls like `find_in_parent_folders()`, `read_terragrunt_config()`).
- Add `before` and `after` configuration to Terragrunt stacks created by `terramate create --all-terragrunt`.

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
  - Add `terramate create --all-terragrunt` option, which will automatically create Terramate stacks for each Terraform module.
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

- Refactor Safeguards

  - Add `disable_safeguards` configuration and `--disable-safeguards` CLI option with possible values
    - `all` Disable ALL safeguards (use with care)
    - `none` Enable ALL safeguards
    - `git` Disable all git related safeguards:
      - `git-untracked` Disable Safeguard that checks for untracked files
      - `git-uncommitted` Disable Safeguard that checks for uncomitted files
      - `git-out-of-sync` Disable Safeguard that checks for being in sync with remote git
    - `outdated-code` Disable Safeguard that checks for outdated code

- Promote cloud commands from `experimental`

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

- Deprecate old safeguard configuration options and issue a warning when used.

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

- Improve the output of `list`, `run` and `create` commands.

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

- Ensure that SIGINT aborts the execution of subsequent stacks in all cases.
- Remove non-supported functions (`tm_sensitive` and `tm_nonsensitive`)

## 0.4.1

### Added

- Add support for globs in the `import.source` attribute to import multiple files at once.
- Add support for listing `unhealthy` stacks with `terramate list --experimental-status=unhealthy`.
- Add support for triggering `unhealthy` stacks with `terramate experimental  trigger --experimental-status=unhealthy`.
- Add support for evaluating `terramate run` arguments with the `--eval` flag.

### Fixed

- Allow to specify multiple tags separated by a comma when using `terramate create --tags` command.
- Fix inconsistent behavior in `terramate create` vs. `terramate create --all-terraform`, both now populate the name/description fields the same way.

## 0.4.0

### Added

- Introduce the `terramate create --all-terraform` command that can be used to initialize plain terraform projects easily with a Terramate Stack configuration.
  Every directory containing Terraform Files that configure either a `terraform.backend {}` block or a `provider {}` block will be initialized.
- Introduce `terramate create --ensure-stack-ids` to add a UUID as stack ID where `stack.id` is not set. (Promoted from `experimental ensure-stack-id`)

### Changed

- BREAKING CHANGE: Make `stack.id` case insensitive. If you used case to identify different stacks, the stack ID need to be adjusted. If you use the recommended UUIDs nothing has to be done.
- Distribute the Terramate Language Server `terramate-ls` alongside `terramate` to be able to pick up changes faster for IDEs.

## 0.3.1

### Added

- Introduce the `terramate experimental cloud info` command.
- Introduce the `--cloud-sync-deployment` flag (experimental).
- Introduce the `terramate experimental ensure-stack-id` command.

### Fixed

- Fix a crash when `generate_hcl {}` has no `content {}` block defined.

## 0.3.0

### Added

- Add the `terramate experimental cloud login` command.

### Fixed

- Fix a file descriptor leak when loading the configuration tree.
