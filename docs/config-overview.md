# Terramate Configuration Overview

Different configurations can be done in Terramate,
ranging from avoiding duplication by leveraging powerful
code generation to flexible orchestration by allowing control
of stacks order of execution.

To do so, Terramate works with configuration files that
have the suffixes:

* `tm`
* `tm.hcl`

Terramate files can be found in any directory of a Terramate project and
all the files in a single directory will be handled as the concatenation
of all of them in a single file, forming a single **configuration**.

A Terramate project is essentially any collection of Terraform code
organized into stacks. The stacks can also reference local modules
inside the project or remote ones.

It is not a hard requirement for Terramate to work that the project uses git 
for version control, but features like change detection do depend on git to
work and will fail if this soft requirement is not met.

In general, a Terramate project looks like this:

* A git project.
* The git top-level dir is the project root dir.
* Stacks are organized as different directories.
* Configuration may be present on any directory.
