// Copyright 2024 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

stack {
  name        = "package printer // import \"github.com/terramate-io/terramate/printer\""
  description = "package printer // import \"github.com/terramate-io/terramate/printer\"\n\nPackage printer defines funtionality for \"printing\" text to an io.Writer e.g.\nos.Stdout, os.Stderr etc. with a consistent style for errors, warnings,\ninformation etc.\n\nvar Stderr = NewPrinter(os.Stderr) ...\ntype Printer struct{ ... }\n    func NewPrinter(w io.Writer) *Printer"
  tags        = ["golang", "printer"]
  id          = "79a0ef5a-2487-4a14-914b-43be60d8d661"
}
