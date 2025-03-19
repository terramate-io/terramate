// Copyright 2025 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

stack {
  name        = "package status // import \"github.com/terramate-io/terramate/cloud/api/status\""
  description = "package status // import \"github.com/terramate-io/terramate/cloud/api/status\"\n\nPackage status provides utilities for parsing Terramate Cloud status filters.\n\nfunc ParseFilters(stackStatus, deploymentStatus, driftStatus string) (resources.StatusFilters, error)"
  tags        = ["api", "cloud", "golang", "status"]
  id          = "c6ea46e4-86f3-4442-80cc-5a62f61dc035"
}
