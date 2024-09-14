// Copyright 2024 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

stack {
  name        = "package filter // import \"github.com/terramate-io/terramate/config/filter\""
  description = "package filter // import \"github.com/terramate-io/terramate/config/filter\"\n\nPackage filter provides helpers for filtering objects.\n\nfunc MatchTags(filter TagClause, tags []string) bool\nfunc MatchTagsFrom(filters []string, tags []string) (bool, error)\ntype Operation int\n    const EQ Operation = iota + 1 ...\ntype TagClause struct{ ... }\n    func ParseTagClauses(filters ...string) (TagClause, bool, error)"
  tags        = ["config", "filter", "golang"]
  id          = "a240182f-0b50-4afe-85c7-56e7e288c91c"
}
