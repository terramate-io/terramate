# Getting Started

## Sign up as a new user

### First Time Sign in

When signing up to the platform at [cloud.terramate.io](https://cloud.terramate.io/), you are asked to choose a social
login provider to sign in with.

Terramate Cloud offers to sign in using:

- A Google Workspace Account (formerly known as GSuite Account),
- A GitHub Account (coming soon; please [get in touch with support](mailto:hello@terramate.io)), or
- A Microsoft Account (coming soon; please [get in touch with support](mailto:hello@terramate.io)).

::: warning
If you are not a registered early access customer yet, your domain might not be allowed-listed yet, and you will need to
[contact our support](mailto:hello@terramate.io) to book a demo and get early access.
:::

### Configuring your Profile

Upon the first sign-in, your profile will need to be created. You need to choose a display name and set your company position. Using your real name or a name your team recognizes is recommended.

### Creating or Joining an Organization

After setting up your profile, you can join an organization you are invited to or create a new one.

You can be a member of multiple organizations simultaneously and thus part of multiple teams.

Select the “join” button to join an organization, and you will become an active member. After this, you can “visit” your newly joined organization.

Creating a new organization is as easy as joining one.

You can choose a display name of your new organization and an organization's short name. The short name will be used in URLs (`https://cloud.terramate.io/o/{short-name}`) when visiting the organization or in Terramate CLI when selecting the organization to sync or receive data from.

### Inviting your team

After creating a new organization, you can invite your teammates by e-mail.

You can select any number of e-mail addresses to invite, or you can skip this step to invite your team later from the Organization Management area.

### The dashboard

Initially, you will be located on the organization's dashboard. If no data has been synchronized to your organization so far, instructions to do so will lead you to this documentation.

## Connecting the CLI

To synchronize the first data with your new Terramate Cloud Organization, you must also sign in from your CLI after signing up with the cloud.

You can use [`terramate cloud login`](../../cli/cmdline/cloud-login.md) to log in to Terramate Cloud. A browser window will allow you to select the Google Workspace Account you want to sign in with.

You need to select the same account you just signed up with to use Terramate CLI with your Terramate Cloud Organization.

You can validate you are connected to the correct Terramate Cloud Organization using [`terramate cloud info`](../../cli/cmdline/cloud-info.md):

::: code-group
```sh [shell]
terramate cloud info
```

```sh [output]
status: signed in
provider: Google Social Provider
user: Your Display Name
email: you@example.com
organizations: example
```
:::

After successful sign-in via Terramate CLI, it is recommended to persist the selected cloud organization to your configuration
by creating a config section in e.g., your `terramate.tm.hcl` file as shown here, but replacing `"example"` with the selected
short name of your organization:

```bash
terramate {
  config {
    cloud {
      organization = "example"
    }
  }
}
```

Terramate CLI is now aware of Terramate Cloud and can be used to synchronize data.

## Synchronizing initial Terraform drift information

For this next step, you need to have a repository containing some Terramate Stacks and access to run a `terraform plan` command. If you have a repository with directories containing plain terraform configuration, you can detect Terraform stacks using `terramate create --all-terraform` and configure them as Terramate Stacks.

Each stack requires a unique Stack ID. If you did not set stack IDs, you can use `terramate create --ensure-stack-ids` to generate an ID for all available stacks. Be sure to git commit your changes if you created stack ids.

It is recommended to execute the initial drift synchronization with clean git status: not having any uncommitted or untracked files and being on a merged and stable state of your IaC.

::: info
The following commands expect your environment or Terraform configuration to allow you to run the terraform plan without any additional steps.
If you are using toolings like [aws-vault](https://github.com/99designs/aws-vault) or another authentication wrapper,
you will need to replace Terraform with your wrapper call, e.g., `aws-vault exec my profile terraform` or wrap the full terramate call.
:::

# On-boarding

1. Execute `terramate run terraform init` to initialize terraform in all available stacks. 
2. Execute `terramate run --cloud-sync-drift-status --cloud-sync-terraform-plan-file=drift.tfplan -- terraform plan -detailed-exitcode -out drift.tfplan` to synchronize your drift plans. Continue reading to understand what will happen before actually executing the command.

Terramate CLI will do the following things when executing the command:

1. `terramate` will `run` a `terraform plan -detailed-exitcode -out drift.tfplan` in every stack available.
    1. The `-detailed-exitcode` will ensure we get information about planned changes. Terraform will exit with an exit code of 
        - `0` in case the plan was created successfully and no changes were planned.
        - `1` in case there have been errors and a plan could not be created.
        - `2` in case the plan was created successfully and changes have been planned.
        
        Terramate CLI will interpret the exit code accordingly and synchronize the status to the cloud.
        
    2. The `-out drift.tfplan` tells Terraform to store the planned changes in a file within each stack called `drift.tfplan`
2. The `--cloud-sync-drift-status` option will ensure Terramate CLI honors the `detailed-exitcode` of Terraform and synchronize the status to Terramate Cloud
3. The `--cloud-sync-terraform-plan-file=drift.tfplan` is an additional option ensuring that Terramate CLI synchronizes the drift details to Terramate Cloud. We are using the same name for the plan file that was created using `terrafrom plan`.
    
    Terramate CLI will redact all sensitive fields before synchronizing the details to Terramate Cloud as a terraform plan file can contain sensitive information. In addition, the Terramate Cloud Backend will redact the values again before persisting them in case someone synchronizes a Terraform Plan without using Terramate CLI.
    

After this command is executed and all stacks have been synchronized, you can visit the dashboard on Terramate Cloud again and see the results of your drift synchronization.

In the best case, you will have zero drifts detected. In any other case, you can review drifts also from the CLI:

1. Start with identifying all drifted stacks by running `terramate list --cloud-status drifted` 
2. If stacks are listed using the previous command, enter a stack (change directory using `cd`) that has a drift and show the drift details using `terramate cloud drift show` from the stacks directory. This command will print the Terraform plan details and is callable by any user that is part of your organization and connected to the Terramate cloud. No additional cloud access is required, this means this command can be executed without having access to run a `terraform plan` allowing to set up a least needed privilege access structure. Any known sensitive information is redacted from the plan.

### Summary of steps required to use Terramate CLI and Cloud

- `terramate create --all-terraform` (optional, if not yet using Terramate Stacks)
- `terramate create --ensure-ids` (optional, if using Terramate Stacks but not having ids configured yet)
- persist cloud organization in Terramate configuration (optional, recommended)
- `terramate cloud login` login to Terramate Cloud, requires signup via web on [cloud.terramate.io](http://cloud.terramate.io) and being an active member of an organization.
- `terramate run terraform init` initialize terraform in all stacks
- `terramate run --cloud-sync-drift-status --cloud-sync-terraform-plan-file=drift.tfplan -- terraform plan -detailed-exitcode -out drift.tfplan` detect drifts in all stacks.
- `terramate list --cloud-status drifted` to see drifted stacks in CLI (optional)
