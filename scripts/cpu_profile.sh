#!/usr/bin/env bash

WORKSPACE_ROOT="$HOME/src"
TM="$WORKSPACE_ROOT/terramate"
IAC="$WORKSPACE_ROOT/iac-gcloud"

delete_terraform_files() {
    pushd "$IAC"
    fd '_terramate.*tf' --exec rm
    printf "Deleted generated terraform files...\n"
    popd
}

build_terramate() {
    pushd "${WORKSPACE_ROOT}/terramate"
    printf "Building Terramate from source...\n"
    make test/build
    printf "Built Terramate\n"
    popd
}

main() {
    build_terramate
    mv "${TM}/bin/test-terramate" "$IAC/terramate"
    delete_terraform_files

    pushd "$IAC"
    printf "Starting profiling run...\n\n"
    ./terramate --cpu-profiling generate
    printf "Profiling run done, moving to '$WORKSPACE_ROOT/terramate.prof'\n"
    mv "./terramate.prof" "$WORKSPACE_ROOT"
    popd
}

main
