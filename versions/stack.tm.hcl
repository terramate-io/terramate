// Copyright 2024 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

stack {
  name        = "package versions // import \"github.com/terramate-io/terramate/versions\""
  description = "package versions // import \"github.com/terramate-io/terramate/versions\"\n\nPackage versions provide helper functions for version constraint matching.\n\nconst ErrCheck errors.Kind = \"version check error\"\nfunc Check(version string, constraint string, allowPrereleases bool) error\nfunc Match(version, constraint string, allowPrereleases bool) (bool, error)"
  tags        = ["golang", "versions"]
  id          = "a885a9ed-eb00-4f79-b342-c7239ebfb5bd"
}
