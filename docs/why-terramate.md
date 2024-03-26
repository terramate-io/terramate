---
title: Why Terramate
description: Learn how Terramate differentiates to other tooling in the market and why you should be using Terramate.
---

# Why Terramate

This page describes existing approaches to managing cloud infrastructure with Infrastructure as Code and compares those
with what we call the "next generation" Infrastructure as Code approach that we provide with Terramate.

## Different approaches to Infrastructure as Code

Adopting, managing, and automating Infrastructure as Code is a time-consuming,
costly, and error-prone process. Currently, two approaches exist:

### The "Do it yourself" approach 💥

This approach describes teams that built a platform around IaC tools such as Terraform and OpenTofu. This approach often
leads to a combination of dozens of tools that solve different problems in automation, collaboration, security, observability,
etc. Following this approach requires dedicated product teams that glue all those tools into an often fragile platform
that doesn't scale.

### The "Purpose-built CI/CD" approach 🤑

This approach is often referred to as "TACOS"—**T**erraform **A**utomation and **C**ollaboration **software**.
These purpose-built CI/CD platforms come with a heavy price tag, are time-consuming to onboard and manage, and usually
require refactoring your existing IaC configurations into specific patterns. Using a dedicated CI/CD platform also requires
you to grant these platforms broad access to your cloud accounts and state files that often contain sensitive information.

## Why Terramate - The "Terramate" approach 💫

We designed Terramate to provide an alternative and new approach to solve common challenges in Infrastructure as Code.
Our philosophy is to integrate with all your existing tools and configurations in a non-intrusive way. The main value add
of Terramate is to add TACOs capabilities to your existing CI/CD.

If you have more questions about Terramate, feel free to [book a demo](https://terramate.io/demo) or join our community
on [Discord](https://terramate.io/discord).
