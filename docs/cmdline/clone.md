---
title: terramate clone - Command
description: With the terramate command you can easily clone stacks.

# prev:
#   text: 'Stacks'
#   link: '/stacks/'

# next:
#   text: 'Sharing Data'
#   link: '/data-sharing/'
---

# Create

**Note:** This is an experimental command and is likely subject to change in the future.

The `clone` command clones a stack. Terramate will automatically update the 
UUID of the cloned stack.

**Note:** Currently, `clone` does not support nested stacks. We will add this
functionality in the future.

## Usage

`terramate experimental clone SOURCE TARGET`

## Examples

Clone a stack `alice` to target `bob`:

```bash
terramate experimental clone stacks/alice stacks/bob
```
