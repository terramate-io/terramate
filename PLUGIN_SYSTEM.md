<!--
Copyright 2025 Terramate GmbH
SPDX-License-Identifier: MPL-2.0
-->

# Terramate Plugin System (gRPC)

This document describes the gRPC-based plugin system used by the Terramate CLI.
It focuses on the open-source host behavior and the generic extension points.

## Overview

Terramate loads plugins as **separate processes** and communicates with them
over gRPC. Plugins are discovered in the user Terramate directory and described
by a `manifest.json` file.

Key properties:

- **Out-of-process**: plugins run in their own process space.
- **gRPC protocol**: host and plugin exchange data over streaming RPCs.
- **Host ownership**: the CLI owns the TTY, filesystem, and project state.
- **Generic extensions**: plugins expose capabilities rather than embedding
  domain logic into the host.

## Where Plugins Live

Plugins are installed under the user Terramate directory:

```
~/.terramate.d/plugins/<plugin-name>/
  manifest.json
  <binary>
```

The CLI discovers plugins by reading their manifests and loading the binary.

## Manifest Format (Essentials)

The manifest describes:

- plugin name and version
- protocol type (`grpc`)
- binary path(s)

The host uses the manifest to start the plugin and query its capabilities.

## Plugin Capabilities

Plugins declare which services they implement. The host uses these to decide
which extension points to activate.

Common services:

- **CommandService**: custom CLI commands (`terramate scaffold`, etc.)
- **HCLSchemaService**: HCL block schemas + parsing hooks
- **LifecycleService**: post-init hooks (e.g., config patching)
- **GenerateService**: override `terramate generate`

## Command Execution (Generic)

For plugin commands, the host starts the plugin and calls:

```
CommandService.ExecuteCommand
CommandService.ExecuteCommandWithInput (for interactive commands)
```

The command stream supports:

- stdout / stderr
- file writes
- exit code
- interactive forms (see below)

## Interactive Forms

The host renders interactive forms using a **generic form protocol**. Plugins
send a `FormRequest` describing fields and options; the host renders the form
and returns a `FormResponse`.

Field types are generic:

- Select / Multi-select
- Text input / Text area
- Confirm

This keeps UI logic in the host while **all domain logic remains in the plugin**.

## HCL Schema Extension

Plugins can extend the HCL parser by providing block schemas. The host uses
these schemas to parse new block types and can delegate parsing of those blocks
back to the plugin.

## Generate Override

If a plugin exposes `GenerateService`, the host **delegates `terramate generate`
entirely to the plugin**. The plugin executes code generation in its own
process and writes files directly to disk. The host only proxies stdout/stderr
and exit codes.

## Local Installation (Development)

Typical local flow for a gRPC plugin:

1. Build plugin binary
2. Install via `terramate plugin add <name> --source <dir>`
3. Use standard CLI commands

The host will load plugins automatically on demand.

## Error Handling

The host treats plugin errors as command failures and surfaces stderr output
to the user. For interactive commands, the host ensures the terminal is
restored even if the plugin returns an error.
