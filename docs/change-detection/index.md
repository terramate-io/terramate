---
title: Change Detection | Terramate
description: Terramate adds powerful capabilities such as code generation, stacks, orchestration, change detection, data sharing and more to Terraform.

prev:
  text: 'Stacks'
  link: '/stacks/'

next:
  text: 'Sharing Data'
  link: '/data-sharing/'
---

# Change Detection

## Introduction 
Managing changes in infrastructure is a pivotal aspect of upholding a robust, secure, and scalable environment. This becomes particularly relevant while utilizing Terraform, a tool typically employed to implement modifications across various stacks.

Although it's a common practice, applying changes indiscriminately to all stacks can lead to unintended consequences and demand considerable time and computational resources. This is where Terramate comes into play.

Designed to enhance Terraform's efficiency, Terramate facilitates efficient change detection by capitalizing on the strengths of Version Control System (VCS), notably Git. This comprehensive guide aims to help you understand how to use Terramate for detecting and applying changes specifically to the impacted resources in your Terraform project.

By adhering to this guide, you will acquire the skills to smartly manage your infrastructure processes and minimize the potential collateral damage that unplanned changes may bring.

## Prerequisites

Before you embark on change detection with Terramate, ensure you have the following prerequisites in place

**Terraform Project**: An understanding of Terraform's basics will prove instrumental in utilizing Terramate to its full potential and comprehending its efficiency.

**Git**: As Terramate extensively depends on [Git](https://git-scm.com/downloads) for change detection, it's crucial to have Git installed and configured on your device. An understanding of Git commands, such as `git diff` and basic branch management, will be beneficial.

Once these prerequisites are met, you're all set to use Terramate. Let's proceed to the next section for installing Terramate on your operating system.


## Install Terramate

Our indepth [installation guide](https://terramate.io/docs/cli/installation) provides a seamless installation process for Terramate. It includes step-by-step instructions that cater to diverse operating system, along with any necessary dependencies or setup requirements. Kindly refer to the guide relevant to your operating system for a successful installation.


## Configuring Terramate

We have compiled a comprehensive document detailing all necessary [configuration settings](https://terramate.io/docs/cli/stacks/) and options for Terramate. This guide offers step-by-step instructions along with code examples for a correct Terramate setup.

Make sure to review the Terramate Configuration Guide to ensure your configuration aligns with the guide before advancing to change detection. Once configured correctly, you're prepared to harness Terramate's capability for efficient change detection in your infrastructure.


# Performing Change Detection


This section provides a walkthrough on using Terramate to detect and apply changes solely to the affected resources in your Terraform project. Follow these instructions to enhance your infrastructure management process and limit the impact of modifications.


## Step 1: Compute Changed Stacks

Leveraging the power of Version Control System (VCS), notably Git, Terramate computes the changed stacks in your project. By comparing the last Terraform applied change revision **baseref** with the current change, Terramate pinpoints the stacks needing modifications.

To compute the changed stacks, follow these instructions:

1. Identify the `baseref` value according to your project's branch:

- If you're on the default branch, the baseref is `HEAD^`.
- If you're on a feature branch, the baseref is `origin/main`.

Run the following command, replacing `baseref` with the appropriate baseref value:

```bash
terramate run --changed --git-change-base "baseref" -- terraform plan
```

This command instructs Terramate to compute the changed stacks by comparing the differences between the baseref revision and the current change. Terramate will then create a plan for implementing the necessary changes to the impacted resources.

For instance;

```console
$ git branch
main
$ git rev-parse HEAD
80e581a8ce8cc1394da48402cc68a1f47b3cc646
$ git pull origin main
...
$ terramate run --changed --git-change-base 80e581a8ce8cc1394da48402cc68a1f47b3cc646 \
    -- terraform plan
```

## Step 2: Module Change Detection

Terraform stacks often comprise multiple local modules. In such scenarios, Terramate has a built-in capability to detect modifications in the modules referenced by the stack. Whenever a referenced module undergoes a change, Terramate flags the corresponding stack as changed, signalling a requirement for re-deployment.

Ensure that your Terraform stack accurately declares its dependencies on local modules. This step facilitates Terramate in tracking changes in the modules effectively and accurately.

The configuration of module change detection in Terramate is demonstrated in the example provided in the main documentation. 

![Module Change Detection](../assets/module-change-detection.gif)

Terramate performs this by parsing all `.tf` files within the stack and examining if the local modules it depends on have been modified.


## Arbitrary Files Change Detection:

Terramate offers a unique feature that enables you to designate watched files. These are the files that, upon being modified, trigger changes in the stack. 

This feature is especially useful when detecting changes in specific files or external dependencies that could influence your Terraform stacks. Let's delve into how you can configure Terramate for this purpose.

To activate arbitrary files change detection in Terramate, follow these steps:

- **Specify Watched Files**: 
Define the files you want Terramate to monitor for changes in your stack configuration. If any of these files undergo modification, Terramate marks the stack as changed, regardless of whether the stack's own code remains the same.

Example:

```bash
stack {
   watch = [
      "/external/file1.txt",
      "/external/file2.txt"
   ]
}
```
In this example, Terramate marks the stack as changed if either `file1.txt` or `file2.txt` within the `/external` directory is modified.

- **External Dependencies**: 
In addition to specifying files, you can also list external dependencies that are crucial to your stack's configuration. Terramate will track changes in these dependencies and mark the stack as changed accordingly.

Configuring Terramate to monitor specific files and external dependencies offers you more precise control over change detection, thereby enabling you to respond swiftly to modifications affecting your Terraform stacks.


## Conclusion

In this guide, we explored the powerful capabilities of Terramate for change detection in your Terraform infrastructure. By following the key steps and concepts outlined, you can streamline your change management process and ensure efficient infrastructure deployment. Let's recap the key points covered:

- **Performing Change Detection**: Terramate utilizes the power of your Version Control System (VCS) to compute changed stacks. By comparing the revision of the last Terraform applied change (baseref) with the current change, Terramate identifies the stacks that require modifications.

- **Module Change Detection**: Terramate detects changes in the modules referenced by your Terraform stacks. When a referenced module undergoes modifications, Terramate marks the corresponding stack as changed, indicating the need for redeployment.

- **Arbitrary Files Change Detection**: Terramate allows you to specify watched files and external dependencies that trigger stack changes when modified. This feature enables you to monitor specific files or dependencies critical to your stack's configuration.

> By leveraging Terramate's change detection capabilities, you can ensure that modifications are applied only to the affected resources, reducing risks and improving the efficiency of your infrastructure management.
>

We encourage you to incorporate Terramate into your Terraform workflow and explore its full potential for streamlined change detection. By utilizing the features discussed in this guide, you can enhance the reliability and maintainability of your infrastructure deployments.

