// Copyright 2024 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

stack {
  name        = "package generate // import \"github.com/terramate-io/terramate/generate\""
  description = "package generate // import \"github.com/terramate-io/terramate/generate\"\n\nPackage generate implements code generation. It includes all available code\ngeneration strategies on Terramate and it also handles outdated code detection\nand deletion.\n\nconst ErrLoadingGlobals errors.Kind = \"loading globals\" ...\nfunc DetectOutdated(root *config.Root, target *config.Tree, vendorDir project.Path) ([]string, error)\nfunc ListGenFiles(root *config.Root, dir string) ([]string, error)\ntype FailureResult struct{ ... }\ntype GenFile interface{ ... }\ntype LoadResult struct{ ... }\n    func Load(root *config.Root, vendorDir project.Path) ([]LoadResult, error)\ntype Report struct{ ... }\n    func Do(root *config.Root, dir project.Path, vendorDir project.Path, ...) Report\ntype Result struct{ ... }"
  tags        = ["generate", "golang"]
  id          = "bceb4fd1-ea6c-4346-bdbc-4817a80c70ae"
}
