# Copyright 2024 Terramate GmbH
# SPDX-License-Identifier: MPL-2.0

packages() {
    go list -f '{{.Dir}}' ./... | sort
}
