---
title: HCL Code Generation | Terramate
description: Terramate adds powerful capabilities such as code generation, stacks, orchestration, change detection, data sharing and more to Terraform.

prev:
  text: 'Code Generation Overview'
  link: '/code-generation/'

next:
  text: 'Generate File'
  link: '/code-generation/generate-file'
---

# Generating HCL Code with Terramate

## Configuration files and `generate_hcl` blocks

To begin generating HCL code with Terramate, developers need to define configuration files that specify the desired code components and their corresponding values. Terramate utilizes the `generate_hcl` block to generate HCL code based on these configurations. Multiple generate_hcl blocks can be defined within a [configuration file](https://terramate.io/docs/cli/configuration/), each responsible for generating a specific code component.

## Understanding the `generate_hcl` block structure

The `generate_hcl` block follows a specific structure, consisting of a label, a file path, and a content block. The label provides a unique identifier for the generated code component, while the file path determines the location where the generated code will be saved. The content block contains the HCL code that will be generated.

## Label and File Path

The label is defined using the label key within the `generate_hcl` block. It should be a descriptive name that helps identify the purpose of the generated code component. 

For example, a label could be `backend_configuration` for generating backend configurations or `provider_configuration` for generating provider configurations.

The file path is specified using the `file_path` key within the `generate_hcl` block. It determines the location and file name of the generated code. Developers can provide a relative or absolute path based on their project structure and requirements.

## Content block for Code Generation

The content block is where the actual HCL code is defined. It uses the content key within the `generate_hcl` block. The content can include HCL code snippets, expressions, interpolations, functions, and references to Terramate-defined data.

Developers can leverage the full power of HCL to define the desired code component within the content block. This includes using variables, loops, conditionals, and other HCL constructs to dynamically generate code based on specific conditions or requirements.

## Generating Backend Configurations for Stacks

Terramate enables the generation of backend configurations for stacks within a project. 

Backend configurations define where Terraform stores its state file and provide access to remote state storage, such as AWS S3, Azure Blob Storage, or HashiCorp Consul.


## Example: Generating a Backend Configuration

Lets generate backend and provider configurations for all stacks inside a project. We can use the following code snippet within a `generate_hcl` block, given these globals defined on the root of the project:

```hcl
globals {
  backend_data = "backend_data"
  provider_data = "provider_data"
  provider_version = "0.6.6"
  terraform_version = "1.1.3"
}
```

We can define the generation of a backend configuration for all stacks by defining a `generate_hcl` block in the root of the project:

```hcl
generate_hcl "backend.tf" {
  content {
    backend "local" {
      param = global.backend_data
    }
  }
}
```

Which will generate code for all stacks, creating a file named `backend.tf` on each stack:

```hcl
backend "local" {
  param = "backend_data"
}
```

## Generating Provider and Terraform Configurations for Stacks

Terramate also facilitates the generation of provider configurations and the main Terraform configuration for stacks within a project. Provider configurations specify the providers to be used and their corresponding configuration settings. 

The main Terraform configuration contains the resource and module definitions required to provision the infrastructure.

## Example: Generating Provider and Terraform Configurations

To generate provider/terraform configuration for all stacks we can add in the root configuration:

```hcl
generate_hcl "provider.tf" {

  content {
    provider "name" {
      param = global.provider_data
    }

    terraform {
      required_providers {
        name = {
          source  = "integrations/name"
          version = global.provider_version
        }
      }
    }

    terraform {
      required_version = global.terraform_version
    }

  }

}
```

Which will generate code for all stacks, creating a file named `provider.tf` on each stack:

```hcl
provider "name" {
  param = "provider_data"
}

terraform {
  required_providers {
    name = {
      source  = "integrations/name"
      version = "0.6.6"
    }
  }
}

terraform {
  required_version = "1.1.3"
}
```

In the next section, we will dive deeper into the process of working with `tm_dynamic` Blocks and explore practical examples.


# Working with `tm_dynamic` Blocks

The `tm_dynamic` block is a specialized block type that is utilized within the content block of the generate_hcl block. 

It shares similarities with Terraform dynamic blocks but it also incorporates partial evaluation of the expanded code. This allows for more flexible and dynamic code generation.

## Partial evaluation of Expanded Code

One of the key features of the `tm_dynamic` block is its ability to perform partial evaluation of the expanded code. 

This means that Terramate variables and functions within the block are evaluated, while other code elements are directly copied to the final generated code without evaluation.

## Usage of Content and Attributes in `tm_dynamic` blocks

The `tm_dynamic` block can be defined using either a content block, an attributes attribute, or a combination of both. 

The content block allows for the generation of additional sub-blocks and nesting of `tm_dynamic` blocks.

## Examples: Generating Dynamic Blocks with `tm_dynamic`

To get the more clear understanding of the concepts that we have explained above, let’s take some examples and understand those examples by looking at the generated outcome.

### Using the Content Block:

```hcl
globals {
  values = ["a", "b", "c"]
}

generate_hcl "file.tf" {
  content {
    tm_dynamic "block" {
      for_each = global.values
      iterator = value
      content {
        attr = "index: ${value.key}, value: ${value.value}"
        attr2 = not_evaluated.attr
      }
    }
  }
}
```

This example demonstrates the usage of the content block within the `tm_dynamic` block. It generates a `file.tf` file with multiple blocks, each containing different values based on the iteration. The Terramate variables `value.key` and `value.value` are evaluated within the block.

Generated `file.tf`:

```hcl
block {
  attr = "index: 0, value: a"
  attr2 = not_evaluated.attr
}

block {
  attr = "index: 1, value: b"
  attr2 = not_evaluated.attr
}

block {
  attr = "index: 2, value: c"
  attr2 = not_evaluated.attr
}
```

### Using Labels Attribute:

```hcl
globals {
  values = ["a", "b", "c"]
}

generate_hcl "file.tf" {
  content {
    tm_dynamic "block" {
      for_each = global.values
      iterator = value
      labels = ["some", "labels", value.value]

      content {
        key = value.key
        value = value.value
      }
    }
  }
}
```

This example showcases the usage of the labels attribute within the `tm_dynamic` block. It generates blocks with custom labels based on the values obtained from the iteration.

Generated `file.tf`:

```hcl
block "some" "labels" "a" {
  key   = 0
  value = "a"
}

block "some" "labels" "b" {
  key   = 1
  value = "b"
}

block "some" "labels" "c" {
  key   = 2
  value = "c"
}
```

### Using Attributes:

```hcl
globals {
  values = ["a", "b", "c"]
}

generate_hcl "file.tf" {
  content {
    tm_dynamic "block" {
      for_each = global.values
      iterator = value

      attributes = {
        attr = "index: ${value.key}, value: ${value.value}"
        attr2 = not_evaluated.attr
      }
    }
  }
}
```

This example demonstrates the usage of the attributes attribute within the `tm_dynamic` block. It generates blocks with custom attributes based on the values obtained from the iteration.

Generated `file.tf`

```hcl
block {
  attr = "index: 0, value: a"
  attr2 = not_evaluated.attr
}

block {
  attr = "index: 1, value: b"
  attr2 = not_evaluated.attr
}

block {
  attr = "index: 2, value: c"
  attr2 = not_evaluated.attr
}
```

### Optional `for_each` Attribute: 
The `for_each` attribute within the `tm_dynamic` block is optional. If not defined, only a single block will be generated, and no iterator will be available during block generation.

### Optional `condition` Attribute:

The `tm_dynamic` block also supports an optional condition attribute that must evaluate to a boolean. If the `condition` is false, the `tm_dynamic` block and its nested `tm_dynamic` blocks are ignored. Other attributes of the `tm_dynamic` block are not evaluated if the `condition` is false.

```hcl
generate_hcl "file.tf" {
  content {
    tm_dynamic "block" {
      for_each = global.values
      condition = tm_can(global.values)
      iterator = value

      attributes = {
        attr = "index: ${value.key}, value: ${value.value}"
        attr2 = not_evaluated.attr
      }
    }
  }
}
```

In this example, if the `global.values` is undefined, the `tm_dynamic` block is ignored during code generation.


# Hierarchical Code Generation

Hierarchical code generation in Terramate allows defining code generation at different levels in a project. It provides flexibility and control over the generated code. 

By specifying code generation rules at various levels, you can achieve granular control over different stacks and the resources that are associated to them.

## Defining Code Generation at different levels in a project

Terramate supports defining code generation rules at different levels within a project structure. This allows you to specify code generation configurations at the root level, module level, or even for individual resources within a module. 

This hierarchical approach enables you to tailor the generated code based on the specific requirements and dependencies of each component.

When defining code generation at respective levels, you can utilize the `generate_hcl` block within the respective configuration files. Each `generate_hcl` block encapsulates the code generation rules for a specific context, such as a module or resource. 

By organizing code generation configurations hierarchically, you can ensure that each component generates the desired code in the correct context.

## Potential impact of Code Generation on multiple or all stacks

One of the key advantages of hierarchical code generation is the ability to apply code generation rules to multiple stacks simultaneously. By defining code generation configurations at a higher level, such as the root level, you can propagate those configurations to all child modules and resources within the project. 

This allows for consistent code generation across multiple stacks, ensuring standardized infrastructure deployments.

When code generation is applied to multiple stacks, any changes or updates made to the code generation configurations at the root level will automatically reflect in all associated stacks during the next generation process. This simplifies maintenance and ensures consistent code generation across the project.

## Restrictions and Conflicts with `generate_hcl` Blocks

While hierarchical code generation provides flexibility, it's important to consider potential restrictions and conflicts that may arise when working with `generate_hcl` blocks. Some of the potential conflicts can come while:

### Overriding Code Generation Configurations: 
When defining code generation configurations at different levels, it's essential to understand the order of precedence. Code generation rules at lower levels, such as module or resource-specific configurations, can override or augment the configurations defined at higher levels. 

It's crucial to carefully manage and review these configurations to avoid conflicts or unintended consequences.

### Conflicting Generate_hcl blocks: 
If multiple `generate_hcl` blocks are defined for the same context, such as multiple generate blocks within the same module, conflicts may occur. It's important to ensure that the code generation rules within the conflicting blocks do not contradict each other or generate conflicting code. 

Resolving such conflicts requires careful evaluation and adjustment of the code generation configurations.

### Dependency Management: 
Hierarchical code generation relies on the dependencies between different components within the project. It's crucial to consider the dependencies and order of code generation to avoid circular dependencies or incomplete code generation. 

Understanding the relationships between modules and resources is essential to ensure the accurate generation of code.


The hierarchical code generation in Terramate provides a powerful mechanism to define code generation configurations at different levels, apply them to multiple stacks, and ensure consistency across your infrastructure deployments. Understanding the hierarchy and managing potential conflicts will enable you to take full advantage of this feature and streamline your infrastructure provisioning process.


# Conditional Code Generation
Conditional code generation allows you to dynamically generate code based on specified conditions. This feature enables you to control the inclusion or exclusion of certain code blocks depending on the evaluated conditions.

## Using the condition attribute for Conditional Code Generation:

To implement conditional code generation, you can utilize the `condition` attribute within the `generate_hcl` block. This attribute accepts a boolean expression that determines whether the code block should be generated or not. 

If the `condition` evaluates to true, the code block will be included in the generated code. If it evaluates to false, the code block will be skipped.

## Evaluating Conditions and generating Code accordingly

During the code generation process, the conditions specified in the `condition` attribute are evaluated. The evaluation can be based on various factors such as input variables, environment settings, or the state of the project.

By leveraging the `condition` attribute, you can dynamically control the generation of code based on different scenarios, making your code more flexible and adaptable to varying conditions.

## Example: Generating Code based on Conditions

Let’s take an example to understand the concept better. 

```hcl
generate_hcl "file" {
  condition = tm_length(global.list) > 0
  content {
    locals {
      list = global.list
    }
  }
}
```

In this example, the `generate_hcl` block includes the condition attribute, which evaluates the length of the variable global.list. If the length of the list is greater than 0, indicating that there are elements in the list, the code blocks within the content block will be generated in the output.

**Let's break down the example:**

* The `condition` attribute is set to `tm_length(global.list) > 0`, where `tm_length()` is a Terramate function that returns the length of a list. If the length of `global.list` is greater than `0`, the `condition` evaluates to `true`.

* Inside the content block, a locals block is defined. This allows you to define local variables that can be used within the code generation process. In this case, a variable named list is defined and assigned the value of `global.list`.

* By utilizing the `condition` attribute, you can selectively generate the code block only when the specified `condition` is met. This allows you to control the inclusion or exclusion of certain code based on the state of the `global.list` variable and thus gives you granular control over your code blocks.


Conditional code generation provides flexibility and adaptability, allowing you to generate code dynamically based on specific conditions. It enables you to tailor your code generation process to meet various scenarios and configurations.

This concludes the subsection on conditional code generation, demonstrating how you can utilize the `condition` attribute to generate code selectively based on specified conditions.


# Partial Evaluation in Code Generation

## Understanding Partial Evaluation Strategy

In HCL code generation, a partial evaluation strategy is employed. This approach allows the generation of code with unknown references or function calls, which are then copied verbatim to the generated code. It enables a flexible and dynamic code generation process.

## Handling Unknown References and Function Calls

When generating HCL code, both unknown references and function calls are handled. Unknown references, such as Terramate references, are retained as is in the generated code. On the other hand, function calls are partially evaluated. 

If a function call starts with the prefix `tm_`, it is considered a Terramate function and will be evaluated. Function calls can have Terramate references or literals as parameters.


# Examples of Partial Evaluation in HCL Code Generation

To understand the above concepts in a better way, lets take some examples.

## Example 1: Mixing Terramate and Terraform references

Assuming we have a single global as Terramate data:

```hcl
globals {
  terramate_data = "terramate_data"
}
```

We can mix Terramate references with Terraform references in our `generate_hcl` block as follows:

```hcl
generate_hcl "main.tf" {
  content {
    resource "myresource" "name" {
      count = var.enabled ? 1 : 0
      data  = global.terramate_data
      path  = terramate.path
      name  = local.name
    }
  }
}
```

This will generate the following `main.tf` file:

```hcl
resource "myresource" "name" {
  count = var.enabled ? 1 : 0
  data  = "terramate_data"
  path  = "/path/to/stack"
  name  = local.name
}
```

The Terramate references `global.terramate_data and terramate.path` are evaluated, while the references to `var.enabled` and `local.name` are retained as is, demonstrating partial evaluation.

## Example 2: Partial Evaluation of Function Calls

Function calls in HCL code generation are partially evaluated. Consider the following code snippet:

```hcl
generate_hcl "main.tf" {
  content {
    resource "myresource" "name" {
      data  = tm_upper(global.terramate_data)
      name  = upper(local.name)
    }
  }
}
```

This will be generated as:

```hcl
resource "myresource" "name" {
  data  = "TERRAMATE_DATA"
  name  = upper(local.name)
}
```

The function call `tm_upper(global.terramate_data)` is evaluated, transforming the value of `global.terramate_data` to uppercase. However, the function call upper `(local.name)` is retained as is since it does not match the Terramate function pattern.

## Example 3: Function call with Terramate reference as parameter

If a parameter of an unknown function call is a Terramate reference, the value of the Terramate reference will be replaced in the function call. For instance:

```hcl
generate_hcl "main.tf" {
  content {
    resource "myresource" "name" {
      data  = upper(global.terramate_data)
      name  = upper(local.name)
    }
  }
}
```

Generates:

```hcl
generate_hcl "main.tf" {
  content {
    resource "myresource" "name" {
      data  = upper("terramate_data")
      name  = upper(local.name)
    }
  }
}
```

Here, the Terramate reference `global.terramate_data` is replaced with its corresponding value, resulting in `upper("terramate_data")`.

Please note that currently, there is no partial evaluation of `for` expressions. Referencing Terramate data inside a `for` expression will result in an error, and `for` expressions with unknown references are copied as it is.