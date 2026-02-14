// Copyright 2026 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

stack {
  name        = "package resolve // import \"github.com/terramate-io/terramate/generate/resolve\""
  description = "package resolve // import \"github.com/terramate-io/terramate/generate/resolve\"\n\nPackage resolve is responsible for resolving and fetching sources for package\nitems.\n\nconst ComponentsDir = \"components\" ...\nfunc CombineSources(src, parentSrc string) string\nfunc NewAPI(cachedir string) di.Factory[API]\ntype API interface{ ... }\ntype Kind int\n    const Bundle Kind = iota ...\ntype Option func(API, *OptionValues)\n    func WithParentSource(parentSrc string) Option\ntype OptionValues struct{ ... }\ntype Resolver struct{ ... }"
  tags        = ["generate", "golang", "resolve"]
  id          = "7933cdde-f6ee-4bf8-b4df-919b89587817"
}
