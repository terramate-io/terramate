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

- Add `terramate [run, script run] --{include,only}-output-dependencies` flags.
  - These flags are needed for initializing the output dependencies of stacks which had its dependencies not changed in the same run.
  - The `--include-output-dependencies` flag includes the output dependencies in the execution order.
  - The `--only-output-dependencies` flag only includes the output dependencies in the execution order.

### Fixed

- Fix the sync of `base_branch` information in the Terramate Cloud deployment.

## v0.11.7

### Added

- Add `--no-generate` to `terramate experimental clone` to skip the generation phase after the clone.

### Fixed

- Fix an edge case crash in `terramate list` when using the `--status` flag.

## v0.11.6

### Fixed

- Fix race condition when using `parallel` in combination with `outputs-sharing`.

## v0.11.5

### Added

- Add support for Terramate Cloud API keys for machine-to-machine communication.
  - You can connect to Terramate Cloud using API keys exported as `TMC_TOKEN` environment variable.
  - The API Key configuration has precedence over all other authentication methods.
- Add support for unprefixed Bitbucket URLs in Terraform `module.source`.
- Add support for Bitbucket Cloud Pipelines in the Terramate Cloud features.

### Fixed

- Fix `terramate list --changed` panic when a Terraform module references a path outside the project root.

## v0.11.4

### Added

- Add support for tracking `file()` usages in Terragrunt files for enhancing the change detection.
  - Now if you have Terragrunt modules that directly read files from elsewhere in the project, Terramate will
  mark the stack changed whenever the aforementioned file changes.
- Add telemetry to collect anonymous usage metrics.
  - This helps us to improve user experience by measuring which Terramate features are used most actively.
    For further details, see [documentation](https://terramate.io/docs/cli/telemetry).
  - Can be turned off by setting `terramate.config.telemetry.enabled = false` in the project configuration,
    or by setting `disable_telemetry = true` in the user configuration.
- Add `create --all-terragrunt --tags a,b,c` for creating all discovered Terragrunt stacks with the given tags.
- Add `create --all-terraform --tags a,b,c` for creating all discovered Terraform stacks with the given tags.

### Fixed

- Fix the command-line parsing of `run` and `script run` which were not failing from unknown flags.
- Fix `create --all-terragrunt` creating Terragrunt stacks with cycles.
- Improve the error reporting of the Outputs Sharing feature.
- Fix crash in the Terragrunt integration when the project had modules with dependency paths outside the current Terramate project.
  - A warning will be shown for such configurations.
- Fix the Terragrunt scanner not supporting nested modules.

## v0.11.3

### Fixed

- Fix Terragrunt modules change detection.

## v0.11.2

### Added

- Add `terramate.stack.parent.id` metadata to stacks that are part of a parent-child hierarchy.

### Fixed

- Nested `map` blocks not rendered if the `value` block contain no attributes.
- Fix `terramate fmt` not respecting the `.tmskip` file.
- Fix the `terramate fmt` not recursively formatting `.tmgen` files.

## v0.11.1

### Added

- Faster code generation through parallelization.
  - Enabled by default, use `terramate generate --parallel <n>` to control the amount of concurrent units (default = number of logical CPU cores).

## v0.11.0 

### Added

- Add `--enable-change-detection=<options>` and `--disable-change-detection=<options>` to the commands: `terramate list`, `terramate run` and `terramate script run`. 
  - These flags overrides both the default change detection strategy and the configuration in `terramate.config.change_detection.git` block.
- Add support for using `TM_ARG_*` environment variables to configure cli commands.
  Note: This is an incremental change. Only global flags and `terramate run` flags were added for now.
  - For example: Use `TM_ARG_CHDIR=stacks/prod TM_ARG_RUN_REVERSE=1 terramate run -- terraform apply` to run from inside `stacks/prod` and with reversed execution order (which is the same as `terramate --chdir stacks/prod run --reverse -- terraform apply`).

### Changed

- **(Breaking change)** The `terramate list --changed` now considers *untracked* and * uncommitted*  files for detecting changed stacks.
  - This behavior can be turned off by `terramate.config.change_detection.git.untracked = "off"` and `terramate.config.change_detection.git.uncommitted = "off"`.
- **(Breaking change)** Remove the deprecated `terramate experimental run-order`.
  - The `terramate list --run-order` was introduced in version `v0.4.5` and provides the same functionality as the removed command.

## v0.10.9

### Added

- Add support for dot (`.`) in the tag syntax.
  - Now you can add tags like `v1.0.0-abc_xyz`

### Changed

- The **Outputs Sharing** feature now has no default value for the `sensitive` field of `input` and `output` blocks.

## v0.10.8

### Fixed

- Fix `trigger --ignore-change` not ignoring stacks changed due to Terraform or Terragrunt changes.
- Fix **Outputs Sharing** feature not generating the `output.sensitive` attribute.
- Fix **Outputs Sharing** feature not generating the `variable.sensitive` attribute.

## v0.10.7

### Added

- Add `terramate create --wants ... --wanted-by ...` flags for configuring the `stack.wants` and `stack.wanted_by` attributes, respectively.

### Fixed

- Fix the cleaning up of orphaned files in the `terramate generate` to respect the `-C <dir>` flag.
- Fix the value of `terramate.stack.path.basename` and `terramate.root.path.basename` which were given the value of `"\\"` in the case the stack was defined at the project root directory.

### Changed

- Several performance improvements in the change detection.


## v0.10.6

### Fixed

- Fix "outdated-code" safeguard giving false positive results for files generated
 in subdirectory of stacks.

## v0.10.5

### Fixed

- Fix `outputs-sharing` experiment to use `type = any` for generated Terraform input variables in dependent stacks.
- Fix `outdated-code` safeguard not working for `generate_file` blocks with `context=root` option.

## v0.10.4

### Added

- Add `tm_hclencode()` and `tm_hcldecode()` functions for encoding and decoding HCL content.

### Fixed

- Fix `go install` by removing a not needed `replace` directive in the `go.mod`.
- Fix `git` URI normalization in case the project path component begins with a number.

## v0.10.3

### Changed

- Use OpenTofu functions instead of Terraform `v0.15.3` functions.

### Added

- Add `tm_strcontains(string, substr)`.
- Add `tm_startswith(string, prefix)`.
- Add `tm_endswith(string, suffix)`.
- Add `tm_timecmp(t1, t2)`.
- Add `tm_cidrcontains(cidr, ip)`
- Add experimental support for `tm_tomlencode` and `tm_tomldecode`.
  - Can be enabled with `terramate.config.experiments = ["toml-functions"]`.

## v0.10.2

### Fixed

- Fix `outputs-sharing` failure cases not synchronizing to Terramate Cloud.

## v0.10.1

### Added

- Add `sharing_backend`, `input` and `output` blocks for the sharing of stack outputs as inputs to other stacks.
  - The feature is part of the `outputs-sharing` experiment and can be enabled by setting `terramate.config.experiments = ["outputs-sharing"]`.

### Fixed

- Fix the repository normalization for Gitlab subgroups.
  - Now it supports repository URLs like `https://gitlab.com/my-company-name/my-group-name/my-other-group/repo-name`.
- Fix a deadlock in the `terramate run` and `terramate script run` parallelism by
  releasing the resources in case of errors or if dry-run mode is enabled.

## v0.10.0

### Added

- Add support for generating files relative to working directory. Both examples below only generate files inside `some/dir`:
  - `terramate -C some/dir generate`
  - `cd some/dir && terramate generate`

## v0.9.5

### Fixed

- Fix the repository normalization for Gitlab subgroups.
  - Now it supports repository URLs like `https://gitlab.com/my-company-name/my-group-name/my-other-group/repo-name`.
- Fix a deadlock in the `terramate run` and `terramate script run` parallelism by
  releasing the resources in case of errors or if dry-run mode is enabled.

## v0.9.4

### Changed

- Use [terramate-io/tfjson](https://github.com/terramate-io/tfjson) in replacement of [hashicorp/terraform-json](https://github.com/hashicorp/terraform-json) for the JSON representation of the plan file.

## v0.9.3

### Fixed

- Fix a missing check for `GITLAB_CI` environment variable.
  - The previews feature can only be used in _Github Actions_ and _Gitlab CI/CD_ and a check for the latter was missing.
- Fix the synchronization of the GitLab Merge Request commit SHA and pushed_at.

## v0.9.2

### Changed

- Reverted: feat: generate files in working directory. (#1756)
  - This change was reverted due to an unaccounted breaking change.

## v0.9.1

### Added

- Add cloud filters `--deployment-status=<status>` and `--drift-status` to commands below:
  - `terramate list`
  - `terramate run`
  - `terramate experimental trigger`
- Add support for generating files relative to working directory. Both examples below only generate files inside `some/dir`:
  - `terramate -C some/dir generate`
  - `cd some/dir && terramate generate`
- Add synchronization of Gitlab Merge Request and CI/CD Metadata.

### Fixed

- Fix crash when supplying a tag list with a trailing comma separator.

## v0.9.0

### Added

- Add support for `[for ...]` and `{for ...}` expressions containing Terramate variables and functions inside the `generate_hcl.content` block.
- Add experimental support for deployment targets. This allows to keep separate stack information when the same stacks are deployed to multiple environments, i.e. production and staging.
  - Can be enabled with `terramate.config.experiments = ["targets"]` and `terramate.config.cloud.targets.enabled = true`.
  - Once enabled, commands that synchronize or read stack information to Terramate Cloud require a `--target <target_id>` parameter. These include:
    - `terramate run --sync-deployment/--sync-drift-status/--sync-preview`
    - `terramate script run`
    - `terramate run --status`
    - `terramate list --status`
    - `terramate cloud drift show`
- Add `script.lets` block for declaring variables that are local to the script.
- Add `--ignore-change` flag to `terramate experimental trigger`, which makes the change detection ignore the given stacks.
  - It inverts the default trigger behavior.
- Add `--recursive` flag to `terramate experimental trigger` for triggering all child stacks of given path.

### Fixed

- Fix `terramate experimental trigger --status` to respect the `-C <dir>` flag.
	- Now using `-C <dir>` (or `--chdir <dir>`) only triggers stacks inside the provided dir.
- Fix the update of stack status to respect the configured parallelism option and only set stack status to be `running` before the command starts.
- Fix `terramate experimental trigger` gives a misleading error message when a stack is not found.

### Changed

- (**BREAKING CHANGE**) The format of the generated code may change while being still semantically the same as before. This change is marked as "breaking", because this may trigger change detection on files where the formatting changes.

## v0.8.4

### Fixed

- Fix limitation preventing `tm_dynamic.attributes` use with map types.
- Fix the loading of `terramate.config.run.env` environment variables not considering equal signs in the value.

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

## v0.6.7

### Fixed

- Fix potential crash when trying to obtain the pull request metadata associated with a Github repository.

## v0.6.6

### Fixed

- Fix issue with handling paginated responses from Github API when retrieving review and GHA action metadata.

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

## v0.4.8

### Fixed

- Fixed inconsistency in `stack.id` case-sensitivity when `--cloud-sync-*` flags are used.

### Changed

- Reverted change in the previous release because the fix didn't address all cases.

## v0.4.7

### Fixed

- Remove lowercase validation from `stack.meta_id` (`stack.id` in the stack block)
  - This would allow users to sync stacks to Terramate Cloud with upper case characters in the `stack.id` attribute.

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
