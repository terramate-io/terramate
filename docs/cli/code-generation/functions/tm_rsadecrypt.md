---
title: tm_rsadecrypt - Functions - Configuration Language
description: The tm_rsadecrypt function decrypts an RSA-encrypted message.
---

# `tm_rsadecrypt` Function

`tm_rsadecrypt` decrypts an RSA-encrypted ciphertext, returning the corresponding
cleartext.

```hcl
tm_rsadecrypt(ciphertext, privatekey)
```

`ciphertext` must be a base64-encoded representation of the ciphertext, using
the PKCS #1 v1.5 padding scheme. Terraform uses the "standard" Base64 alphabet
as defined in [RFC 4648 section 4](https://tools.ietf.org/html/rfc4648#section-4).

`privatekey` must be a PEM-encoded RSA private key that is not itself
encrypted.

Terramate has no corresponding function for _encrypting_ a message. Use this
function to decrypt ciphertexts returned by remote services using a keypair
negotiated out-of-band.

## Examples

```sh
tm_rsadecrypt(tm_filebase64("${path.module}/ciphertext"), tm_file("privatekey.pem"))
Hello, world!
```
