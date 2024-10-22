#!/bin/bash
# Copyright 2024 Terramate GmbH
# SPDX-License-Identifier: MPL-2.0


source "$(dirname "$0")/packages.sh"

rootdir=$(git rev-parse --show-toplevel)

for pkg in $(packages); do
    cd $pkg
    projdir=${pkg#"$rootdir"}
    if [ "x${projdir}" == "x" ]; then
        projdir="/"
    fi
    if ! test -f stack.tm -o -f stack.tm.hcl; then
        echo "Stack ${projdir} must be created! Please run ./hack/create-stacks.sh"
        exit 1
    fi
done
