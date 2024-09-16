// Copyright 2024 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

stack {
  name        = "package event // import \"github.com/terramate-io/terramate/event\""
  description = "package event // import \"github.com/terramate-io/terramate/event\"\n\nPackage event implements a simple event stream and defines all events generated\nby different parts of Terramate.\n\ntype Stream[T any] chan T\n    func NewStream[T any](buffsize int) Stream[T]\ntype VendorProgress struct{ ... }\ntype VendorRequest struct{ ... }"
  tags        = ["event", "golang"]
  id          = "3bef7cce-e755-49c2-ac71-39f7a0e9144f"
}
