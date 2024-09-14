// Copyright 2024 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

stack {
  name        = "package download // import \"github.com/terramate-io/terramate/modvendor/download\""
  description = "package download // import \"github.com/terramate-io/terramate/modvendor/download\"\n\nPackage download is responsible for downloading vendored modules.\n\nconst ErrAlreadyVendored errors.Kind = \"module is already vendored\" ...\nfunc HandleVendorRequests(rootdir string, vendorRequests <-chan event.VendorRequest, ...) <-chan Report\nfunc MergeVendorReports(reports <-chan Report) <-chan Report\ntype IgnoredVendor struct{ ... }\ntype ProgressEventStream event.Stream[event.VendorProgress]\n    func NewEventStream() ProgressEventStream\ntype Report struct{ ... }\n    func NewReport(vendordir project.Path) Report\n    func Vendor(rootdir string, vendorDir project.Path, modsrc tf.Source, ...) Report\n    func VendorAll(rootdir string, vendorDir project.Path, tfdir string, ...) Report\ntype Vendored struct{ ... }"
  tags        = ["download", "golang", "modvendor"]
  id          = "8037655d-1451-44a8-84aa-be1b4f2a4e7d"
}
