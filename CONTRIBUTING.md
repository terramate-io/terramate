# Contributing to Terramate

## A Word Before We Begin

First and foremost, we'd like to express our gratitude to you for taking the time to contribute. We don't accept feature contributions by the community as we are currently sticking to an internal roadmap. This may change in the future.

We welcome and appreciate all bug fix contributions via [Pull Requests](https://github.com/terramate-io/terramate/pulls) along the [GitHub Flow](https://guides.github.com/introduction/flow/).

Thanks!

## Contribution Workflow

For bug reports or requests, please submit your issue in the appropriate repository.

We advise that you open an issue and ask the [CODEOWNERS](https://help.github.com/en/github/creating-cloning-and-archiving-repositories/about-code-owners) and community prior to starting a contribution. This is your chance to ask questions and receive feedback before writing (potentially wrong) code. We value the direct contact with our community a lot, so don't hesitate to ask any questions.

For contributing to Terramate, please follow these steps:

1. Within your fork of
   [Terramate](https://github.com/terramate-io/terramate), create a
   branch for your contribution. Use a meaningful name.
2. Create your contribution, meeting all
   [contribution quality standards](#contribution-quality-standards)
3. [Create a pull request](https://help.github.com/articles/creating-a-pull-request-from-a-fork/)
   against the main branch of the Terramate repository.
4. Work with your reviewers to address any comments and obtain a minimum of 1 approval.
5. Once the pull request is approved, one of the maintainers will merge it.

## Contribution Quality Standards

Most quality and style standards are enforced automatically during integration
testing. Your contribution needs to meet the following standards:

- Each contribution must have a single scope: one feature/bugfix/chore per PR
   - _Exceptions can be accepted if accompanied by good reasoning_
- Include tests for any new functionality (or bug fix) in your pull request.
- Document all your public functions.
- When opening the PR, follow the instructions in the description template.
- If you need an early review, even if not ready, [mark it as Draft](https://docs.github.com/en/pull-requests/collaborating-with-pull-requests/proposing-changes-to-your-work-with-pull-requests/changing-the-stage-of-a-pull-request).
- When ready to be reviewed, squash all commits into a single [signed commit](https://docs.github.com/en/authentication/managing-commit-signature-verification/signing-commits) following the [conventional commits](https://www.conventionalcommits.org/en/v1.0.0/) convention.
