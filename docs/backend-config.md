# Backend Configuration

Terrastack provides some facilities to improve on how backend configuration
is managed on terraform. The idea is to circumvent some of the limitations
from terraform that makes it really hard to avoid duplication/mistakes
when managing backend configuration.

There is no way to define a single parametrized backend configuration
that can then be re-used across different stacks/environments/etc.

You can't even use a local variable as a parameter on the backend config,
from [terraform docs](https://www.terraform.io/docs/language/settings/backends/configuration.html):

```
A backend block cannot refer to named values
(like input variables, locals, or data source attributes).
```

With those limitations in mind, terrastack provides a way to:

* Define a single parametrized backend config and re-use it on multiple stacks.
* Use terrastack metadata, like stack name/path, on the backend config.
* Use global variables on the backend config.

This is done by defining a **backend** block, very similarly to how you would
do on terraform, but inside a **terrastack** block, like this:

```hcl
terrastack {
  backend "type" {
    param = "value"
  }
}
```

## Basic Usage

## Overriding Configuration

## Using metadata

## Using globals
