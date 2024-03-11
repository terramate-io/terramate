# Terragrunt Integration

**Note:** This is an upcoming experimental feature that is subject to change in the future. To use it now, you must enable the project config option `terramate.config.experiments = ["terragrunt"]`.

Terramate now features built-in change detection for Terragrunt projects. This integration allows you to quickly identify which Terragrunt stacks changed based on the structure of the configuration.

## How it works

In addition to the normal stack change detection, Terramate parses the Terragrunt configuration and also identify the cases below:

- included files (by processing `include` blocks)
- dependencies (`dependency` and `dependencies` blocks)
- file read by function calls (`read_terragrunt_config()`, `read_tfvars_file()` and etc)
- local `terraform.source` blocks.
- etc

## Usage

Simply enable it in the experiments configuration:

```hcl
terramate {
  config {
    experiments = ["terragrunt"]
  }
}
```
