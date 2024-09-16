# Copyright 2024 Terramate GmbH
# SPDX-License-Identifier: MPL-2.0

packages() {
    go list -f '{{.Dir}}' ./... | sort
}

packages_with_tests() {
    go list -f '{{.Dir}}' ./... | sort
}
