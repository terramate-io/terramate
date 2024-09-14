#!/bin/bash
# Copyright 2024 Terramate GmbH
# SPDX-License-Identifier: MPL-2.0


source "$(dirname "$0")/packages.sh"

rootdir=$(git rev-parse --show-toplevel)

for pkg in $(packages_with_tests); do
    cd $pkg
    projdir=${pkg#"$rootdir"}
    if [ "x${projdir}" == "x" ]; then
        projdir="/"
    fi
    if test -f stack.tm -o -f stack.tm.hcl; then
        continue
    fi
    name=$(go doc . | head -n1)
    desc=$(go doc .)
    tags=$(terramate experimental eval 'tm_join(",", tm_distinct([for p in tm_split("/", "'$projdir'") : p if p != ""]))')
    if ! test -z $tags; then
        tags="--tags=golang,${tags}"
    else
        tags="--tags=golang"
    fi
    terramate create .          \
        --name="$name"          \
        --description="$desc"   \
        $tags
done
