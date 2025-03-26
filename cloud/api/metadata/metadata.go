// Copyright 2023 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

// Package metadata contains data structures for platform metadata that is sent to TMC,
// i.e. information about CI/CD environments, pull requests, VCS details.
// A large chunk of definitions can also be found in terramate/cloud/types.go.
//
// How the metadata has been handled historically:
//   - Initially, it was a flat string->string map with key prefixes for grouping and simple values.
//   - For PR data, we needed more complex data structures that can hold lists etc, so a separate API object
//     review_request was introduced, which did both hold new data, but also implement some logic on how to abstract
//     pull requests from different structures under a single concept.
//
// In the future, we would like to move away from this and use the following approach:
//   - Use a single API object. We keep using the existing metadata map, but relax it to accept string->any.
//   - Group related data by storing them under a top-level key in metadata.
//     No longer flatten data types into prefixed keys.
//   - Do not abstract between different platforms on the CLI level, instead send the data as-is,
//     i.e. "github_pull_request": {...}, "gitlab_merge_request": {...}, each having different definitions.
package metadata
