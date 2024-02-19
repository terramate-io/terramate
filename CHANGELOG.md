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

- Add `terramate.config.generate.hcl_magic_header_comment_style` option for setting the generated comment style.
- Add support for formatting specific files and stdin (`terramate fmt [file...]` or `terramate fmt -`).
- Add `--cloud-status=status` flag to both `terramate run` and `terramate script run`.
- Add `terramate create --all-terragrunt`.

### Fixed

- Fix language server panic when root directory contain errors.
- (**BREAKING CHANGE**) Fix the execution order when using `tag:` filter in `after/before` in conjunction with implicit filesystem order. Please check the `terramate list --run-order` after
upgrading.

## 0.4.5

### Added

- Add support for `stack_filter` in `generate_file` blocks.
- Promote cloud commands
  - `terramate experimental cloud login` -> `terramate cloud login`
  - `terramate experimental cloud info` -> `terramate cloud info`
  - `terramate experimental cloud drift show` -> `terramate cloud drift show`
- Promote `--experimental-status` flag to `--cloud-status` flag
  - `terramate experimental trigger --experimental-status=` -> `terramate experimental trigger --cloud-status=`
  - `terramate list --experimental-status=` -> `terramate list --cloud-status=`
- Add `list --run-order` flag to list stacks in the order they would be executed.
- Add support for deployment syncing to script commands.
- Add `disable_safeguards` configuration option and CLI flag.
- Add `--detailed-exit-code` to fmt command
- Add `--detailed-exit-code` to generate command

### Fixed

- Fix `tm_dynamic.attributes` being wrapped many times leading to stack exhaustion when cloning expressions in projects with lots of stacks.
- Stack ordering not respected in the `script run`.
- Fix `script.job.command[s]` not handling (typed) lists.

## 0.4.4

### Added

- Add `terramate.config.experiments` configuration to enable experimental features.
- Add support for statuses `ok, failed, drifted and healthy` to the `--experimental-status` flag.
- Add experimental `script` configuration block.
- Add `terramate script list` to list scripts visible in current directory.
- Add `terramate script tree` to show a tree view of scripts visible in current directory.
- Add `terramate script info <scriptname>` to show details about a script.
- Add `terramate script run <scriptname>` to run a script in all relevant stacks.
- Add `stack_filter` block to `generate_hcl` for path-based conditional generation.
- Promote experimental commands
  - `terramate debug show metadata`
  - `terramate debug show globals`
  - `terramate debug show generate-origins`
  - `terramate debug show runtime-env`
- Improvements in the output of `list`, `run` and `create` commands.

### Fixed

- fix(generate): blocks with context=root were ignored if defined in stacks.
- fix: experimental eval/partial-eval/get-config-value wrongly interprets the output as a formatter.
- fix: change detector cannot read user's git config

## 0.4.3

### Added

- Add `--cloud-sync-terraform-plan-file=<plan>` flag for synchronizing the plan
file in rendered ASCII and JSON (sensitive information removed).
- Add configuration attribute `terramate.config.cloud.organization` to select which cloud organization to use when syncing with Terramate Cloud.
- Add sync of logs to _Terramate Cloud_ when using `--cloud-sync-deployment` flag.
- Add `terramate experimental cloud drift show` for retrieving drift details from Terramate Cloud.
- Add support for cloning nested stacks to `terramate experimental clone`. It can also be used to clone directories that
are not stacks themselves, but contain stacks in sub-directories.

### Fixed

- Missing file ranges in the parsing errors of some stack block attributes.

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
- Add support for listing *unhealthy* stacks with `terramate list --experimental-status=unhealthy`.
- Add support for triggering *unhealthy* stacks with `terramate experimental  trigger --experimental-status=unhealthy`.
- Add support for evaluating `terramate run` arguments with the `--eval`
flag.

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
