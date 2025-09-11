// Copyright 2025 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

stack {
  name        = "package di // import \"github.com/terramate-io/terramate/di\""
  description = "package di // import \"github.com/terramate-io/terramate/di\"\n\nfunc Bind[Ifc, Impl any](b *Bindings, factory func(context.Context) (Impl, error)) error\nfunc Get[Ifc any](ctx context.Context) (Ifc, error)\nfunc InitAll(b *Bindings) error\nfunc Override[Ifc, Impl any](b *Bindings, factory func(context.Context, Ifc) (Impl, error)) error\nfunc Require[Ifc any](b *Bindings)\nfunc Validate(b *Bindings) error\nfunc WithBindings(ctx context.Context, b *Bindings) context.Context\ntype Bindings struct{ ... }\n    func NewBindings(ctx context.Context) *Bindings"
  tags        = ["di", "golang"]
  id          = "e935fa1c-06cd-423c-bcc4-f0df1890cbf7"
}
