<p align="center">
  <img src="https://raw.githubusercontent.com/mineiros-io/brand/16aa786a3cd6d0ae2fb89ed756f96c695d0f88e1/terramate-logo.svg" width="160px" align="center" alt="Terramate Logo" />
  <h1 align="center">Terramate</h1>
  <p align="center">
    ✨ <a href="https://terramate.io/docs">https://terramate.io</a> ✨
    <br/>
      Terramate adds powerful capabilities such as code generation, stacks, orchestration, change detection, globals and more to Terraform.
  </p>
</p>
<br/>

<p align="center">
  <a href="https://github.com/mineiros-io/terramate/releases"><img src="https://img.shields.io/github/v/release/mineiros-io/terramate?color=%239F50DA&display_name=tag&label=Version" alt="Latest Release" /></a>
  <a href="https://pkg.go.dev/github.com/mineiros-io/terramate"><img src="https://pkg.go.dev/badge/github.com/mineiros-io/terramate" alt="Go Docs" /></a>
  <a href="https://goreportcard.com/report/github.com/mineiros-io/terramate"><img src="https://goreportcard.com/badge/github.com/mineiros-io/terramate" alt="Go Report Card" /></a>
  <a href="https://codecov.io/gh/mineiros-io/terramate"><img src="https://codecov.io/gh/mineiros-io/terramate/branch/main/graph/badge.svg?token=gMRUkVUAQ4" alt="Code Coverage" /></a>
  <a href="https://github.com/mineiros-io/terramate/actions?query=branch%3Amain"><img src="https://github.com/mineiros-io/terramate/actions/workflows/ci.yml/badge.svg" alt="Terramate CI Status" /></a>
  <a href="https://github.com/mineiros-io/terramate/stargazers" rel="nofollow"><img src="https://img.shields.io/github/stars/mineiros-io/terramate" alt="Stars"></a>
  <a href="https://terramate.io/discord" rel="nofollow"><img src="https://img.shields.io/discord/1088753599951151154?label=Discord&logo=discord&logoColor=white" alt="Discord Server"></a>
</p>

<p align="center">
  <a href="https://terramate.io/docs">📖 Documentation</a> | <a href="https://play.terramate.io">⚡ Playground</a> | <a href="https://terramate.io/docs/cli/getting-started">🚀 Getting Started</a> | <a href="https://terramate.io/discord" title="Slack invite">🙌 Join Us</a>
</p>

<br>
<br>

## Understanding Terramate

- Interested in why we invented Terramate? Read our introduction blog ["Introducing Terramate"](https://blog.mineiros.io/introducing-terramate-an-orchestrator-and-code-generator-for-terraform-5e538c9ee055?source=friends_link&sk=5272c487ef709c80a34d0b451590f263).
- Interested in how Terramate compares to Terragrunt? Read our blog post ["Terramate and Terragrunt"](https://blog.mineiros.io/terramate-and-terragrunt-f27f2ec4032f?source=friends_link&sk=8834b3de00d4af4744aac63051ff3b53).

## Use cases

Terramate helps you to:

- **Keep your code DRY**: Avoid duplication by easily sharing data across your project.
- **Code Generation**: Generate valid Terraform Code to ensure that you can always enter a stack to run plain Terraform commands.
- **Stack Change detection**: Only execute commands in stacks that have been changed in the current branch or since the last merge.
- **Module Change detection**: Enhanced Change Detection allows to identifying stacks that have changes in local modules.
- **Execute Any Command**: Terramate is not a wrapper of Terraform but can execute any commands in (changed) stacks.
- **Execution Order**: Explicitly define an order of execution of stacks.
- **Forced Stack Execution**: Ensure specific stacks are run alongside other stacks.
- **Pure HCL**: All configuration of Terramate can be supplied in the well-known [Hashicorp Configuration Language (HCL)](https://github.com/hashicorp/hcl).

## Documentation

- [Getting Started](docs/getting-started.md)
- [Why Stacks](docs/why-stacks.md)
- [Change Detection](docs/change-detection.md)
- [Config Overview](docs/configuration.md)
- [Configuring a Project](docs/project-config.md)
- [Functions](docs/functions.md)
- [Sharing Data](docs/sharing-data.md)
- [Code Generation](docs/codegen/overview.md)
- [Orchestrating Stacks Execution](docs/orchestration.md)
- Guides (coming soon)

## Join the community

- Join us on [Discord](https://discord.gg/CyzcScEPkc)
- Contact us via email at [hello@mineiros.io](mailto:hello@mineiros.io)

## Reporting bugs and contributing code

- Want to report a bug or request a feature? Open an [issue](https://github.com/mineiros-io/terramate/issues/new)
  <!-- - Want to help us build Terramate? Check out the [Contributing Guide]() -->
  <!-- ## Code of Conduct -->
