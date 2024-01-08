---
title: Delete Stacks
description: Learn how to delete and cleanup stacks in Terramate.
---

# Delete Stacks

Terramate stacks are Infrastructure as Code agnostic, meaning that stacks don't care if you manage Terraform or
something such as Kubernetes manifests in a stack.

You can easily remove a stack from the filesystem without breaking Terramate. However, Terramate will not take care of cleaning up resources and state.
This means you must first decommission your infrastructure resources and state before deleting a stack.

## Cleaning up Terraform Stacks

The recommended method is to run `terraform destroy` in a stack to destroy all resources managed with Terraform, e.g.:

```sh
terramate run -C path/to/stack -- terraform destroy
```

Alternatively, only remove the Terraform resource configuration and run `terraform apply` one before removing the provider
and backend configuration. If you were to remove both in one PR, Terraform would not recognize this and never remove the resources.
