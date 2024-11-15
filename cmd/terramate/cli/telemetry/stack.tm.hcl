// Copyright 2024 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

stack {
  name        = "package telemetry // import \"github.com/terramate-io/terramate/cmd/terramate/cli/telemetry\""
  description = "package telemetry // import \"github.com/terramate-io/terramate/cmd/terramate/cli/telemetry\"\n\nvar DefaultRecord = NewRecord()\nfunc Endpoint() url.URL\nfunc GenerateOrReadSignature(cpsigfile, anasigfile string) (string, bool)\nfunc GenerateSignature() string\nfunc ReadSignature(p string) string\nfunc SendMessage(msg *Message, p SendMessageParams) <-chan error\ntype AuthType int\n    const AuthNone AuthType = iota ...\n    func DetectAuthTypeFromEnv(credpath string) AuthType\ntype Message struct{ ... }\ntype MessageOpt func(msg *Message)\n    func BoolFlag(name string, flag bool, ifCmds ...string) MessageOpt\n    func Command(cmd string) MessageOpt\n    func DetectFromEnv(cmd string, credfile, cpsigfile, anasigfile string) MessageOpt\n    func StringFlag(name string, flag string, ifCmds ...string) MessageOpt\ntype PlatformType int\n    const PlatformLocal PlatformType = iota ...\n    func DetectPlatformFromEnv() PlatformType\ntype Record struct{ ... }\n    func NewRecord() *Record\ntype SendMessageParams struct{ ... }"
  tags        = ["cli", "cmd", "golang", "telemetry", "terramate"]
  id          = "457dc87e-6067-4a10-ab63-b2338e74896b"
}
