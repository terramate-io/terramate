---
title: tm_cidrnetmask - Functions - Configuration Language
description: |-
  The tm_cidrnetmask function converts an IPv4 address prefix given in CIDR
  notation into a subnet mask address.
---

# `tm_cidrnetmask` Function

`tm_cidrnetmask` converts an IPv4 address prefix given in CIDR notation into
a subnet mask address.

```hcl
tm_cidrnetmask(prefix)
```

`prefix` must be given in IPv4 CIDR notation, as defined in
[RFC 4632 section 3.1](https://tools.ietf.org/html/rfc4632#section-3.1).

The result is a subnet address formatted in the conventional dotted-decimal
IPv4 address syntax, as expected by some software.

CIDR notation is the only valid notation for IPv6 addresses, so `cidrnetmask`
produces an error if given an IPv6 address.

::: info
As a historical accident, this function interprets IPv4 address
octets that have leading zeros as decimal numbers, which is contrary to some
other systems which interpret them as octal. We have preserved this behavior
for backward compatibility, but recommend against relying on this behavior.
:::

## Examples

```sh
tm_cidrnetmask("172.16.0.0/12")
255.240.0.0
```
