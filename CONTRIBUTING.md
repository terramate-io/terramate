# Contributing to Terramate

## A Word Before We Begin

You're welcome to send PRs! If we have time, or if the PRs are very small or fix
bugs, we may even integrate them in the near future. But be aware that we might
not get to it for a while, by which time it might no longer be relevant.

Also we will not be able to review and integrate PRs for
new features from the community, since those tend to be larger.
This will change in time.

If you want to ask about whether a PR is consistent with our short-term plan
_before_ you put in the work -- and you should! -- Create an issue on the project
so we can discuss it together.

Feature requests and bug reports in form of issues are also welcomed.

Thanks!

## Contribution Workflow

Terramate uses the “fork-and-pull” development model. Follow these steps if
you want to merge your changes to Terramate:

1. Within your fork of
   [Terramate](https://github.com/mineiros-io/terramate), create a
   branch for your contribution. Use a meaningful name.
1. Create your contribution, meeting all
   [contribution quality standards](#contribution-quality-standards)
1. [Create a pull request](https://help.github.com/articles/creating-a-pull-request-from-a-fork/)
   against the main branch of the Terramate repository.
1. Work with your reviewers to address any comments and obtain a
   minimum of 1 approval.
1. Once the pull request is approved, one of the maintainers will merge it.

## Contribution Quality Standards

Most quality and style standards are enforced automatically during integration
testing. Your contribution needs to meet the following standards:

- Separate each **logical change** into its own commit.
- Include tests for any new functionality (or bug fix) in your pull request.
- Document all your public functions.
- Document your pull requests. Include the reasoning behind each change, and
  the testing done.
