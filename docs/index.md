<p align="center">
  <img src="https://raw.githubusercontent.com/mineiros-io/brand/16aa786a3cd6d0ae2fb89ed756f96c695d0f88e1/terramate-logo.svg" width="160px" align="center" alt="Terramate Logo" />
  <h1 align="center">Terramate</h1>
  <p align="center">
    ✨ <a href="https://terramate.io">https://terramate.io</a> ✨
    <br/>
      Terramate adds powerful capabilities such as code generation, stacks, orchestration, change detection, globals and more to Terraform.
  </p>
</p>
<br/>


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
- [Config Overview](docs/config-overview.md)
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
