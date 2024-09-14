// Copyright 2024 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

stack {
  name        = "package safeguard // import \"github.com/terramate-io/terramate/safeguard\""
  description = "package safeguard // import \"github.com/terramate-io/terramate/safeguard\"\n\nPackage safeguard provides types and methods for dealing with safeguards\nkeywords.\n\ntype Keyword string\n    const All Keyword = \"all\" ...\ntype Keywords []Keyword\n    func FromStrings(strs []string) (Keywords, error)"
  tags        = ["golang", "safeguard"]
  id          = "5b526d39-4c1a-4d18-9f61-5302e8d88fdc"
}
