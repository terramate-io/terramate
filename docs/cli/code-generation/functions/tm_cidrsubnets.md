---
title: tm_cidrsubnets - Functions - Configuration Language
description: |-
  The tm_cidrsubnets function calculates a sequence of consecutive IP address
  ranges within a particular CIDR prefix.
---

# `tm_cidrsubnets` Function

`tm_cidrsubnets` calculates a sequence of consecutive IP address ranges within
a particular CIDR prefix.

```hcl
tm_cidrsubnets(prefix, newbits...)
```

`prefix` must be given in CIDR notation, as defined in
[RFC 4632 section 3.1](https://tools.ietf.org/html/rfc4632#section-3.1).

The remaining arguments, indicated as `newbits` above, each specify the number
of additional network prefix bits for one returned address range. The return
value is therefore a list with one element per `newbits` argument, each
a string containing an address range in CIDR notation.

For more information on IP addressing concepts, see the documentation for the
related function [`tm_cidrsubnet`](./tm_cidrsubnet.md). `tm_cidrsubnet` calculates
a single subnet address within a prefix while allowing you to specify its
subnet number, while `tm_cidrsubnets` can calculate many at once, potentially of
different sizes, and assigns subnet numbers automatically.

When using this function to partition an address space as part of a network
address plan, you must not change any of the existing arguments once network
addresses have been assigned to real infrastructure, or else later address
assignments will be invalidated. However, you _can_ append new arguments to
existing calls safely, as long as there is sufficient address space available.

This function accepts both IPv6 and IPv4 prefixes, and the result always uses
the same addressing scheme as the given prefix.

::: info
As a historical accident, this function interprets IPv4 address
octets that have leading zeros as decimal numbers, which is contrary to some
other systems which interpret them as octal. We have preserved this behavior
for backward compatibility, but recommend against relying on this behavior.
:::

## Examples

```sh
tm_cidrsubnets("10.1.0.0/16", 4, 4, 8, 4)
[
  "10.1.0.0/20",
  "10.1.16.0/20",
  "10.1.32.0/24",
  "10.1.48.0/20",
]

tm_cidrsubnets("fd00:fd12:3456:7890::/56", 16, 16, 16, 32)
[
  "fd00:fd12:3456:7800::/72",
  "fd00:fd12:3456:7800:100::/72",
  "fd00:fd12:3456:7800:200::/72",
  "fd00:fd12:3456:7800:300::/88",
]
```

You can use nested `tm_cidrsubnets` calls with
[`for` expressions](https://developer.hashicorp.com/terraform/language/expressions/for)
to concisely allocate groups of network address blocks:

```sh
tm_[for cidr_block in cidrsubnets("10.0.0.0/8", 8, 8, 8, 8) : cidrsubnets(cidr_block, 4, 4)]
[
  [
    "10.0.0.0/20",
    "10.0.16.0/20",
  ],
  [
    "10.1.0.0/20",
    "10.1.16.0/20",
  ],
  [
    "10.2.0.0/20",
    "10.2.16.0/20",
  ],
  [
    "10.3.0.0/20",
    "10.3.16.0/20",
  ],
]
```

## Related Functions

* [`tm_cidrhost`](./tm_cidrhost.md) calculates the IP address for a single host
  within a given network address prefix.
* [`tm_cidrnetmask`](./tm_cidrnetmask.md) converts an IPv4 network prefix in CIDR
  notation into netmask notation.
* [`tm_cidrsubnet`](./tm_cidrsubnet.md) calculates a single subnet address, allowing
  you to specify its network number.
