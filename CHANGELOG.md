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

- Add `--cloud-sync-drift-status` flag for syncing the status of drift detection
  to the Terramate Cloud.

### Fixed

- Ensured that SIGINT aborts execution of subsequent stacks in all cases.

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
