// Copyright 2023 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package out_test

import (
	"bytes"
	"testing"

	"github.com/madlambda/spells/assert"
	"github.com/terramate-io/terramate/cmd/terramate/cli/out"
)

func TestOutput(t *testing.T) {
	stdout := bytes.Buffer{}
	stderr := bytes.Buffer{}
	oV0 := out.New(0, &stdout, &stderr)
	oV1 := out.New(1, &stdout, &stderr)
	oV2 := out.New(2, &stdout, &stderr)
	oV3 := out.New(3, &stdout, &stderr)

	oV0.MsgStdOut("0msg")       // print
	oV0.MsgStdOutV("0msgV")     // no print
	oV0.MsgStdOutVV("0msgVV")   // no print
	oV0.MsgStdOutVVV("0msgVVV") // no print

	oV1.MsgStdOut("1msg")       // print
	oV1.MsgStdOutV("1msgV")     // print
	oV1.MsgStdOutVV("1msgVV")   // no print
	oV1.MsgStdOutVVV("1msgVVV") // no print

	oV2.MsgStdOut("2msg")       // print
	oV2.MsgStdOutV("2msgV")     // print
	oV2.MsgStdOutVV("2msgVV")   // print
	oV2.MsgStdOutVVV("2msgVVV") // no print

	oV3.MsgStdOut("3msg")       // print
	oV3.MsgStdOutV("3msgV")     // print
	oV3.MsgStdOutVV("3msgVV")   // print
	oV3.MsgStdOutVVV("3msgVVV") // print

	oV0.MsgStdOutV0("0msgV0") // print
	oV0.MsgStdOutV1("0msgV1") // no print
	oV0.MsgStdOutV2("0msgV2") // no print
	oV0.MsgStdOutV3("0msgV3") // no print

	oV1.MsgStdOutV0("1msgV0") // no print
	oV1.MsgStdOutV1("1msgV1") // print
	oV1.MsgStdOutV2("1msgV2") // no print
	oV1.MsgStdOutV3("1msgV3") // no print

	oV2.MsgStdOutV0("2msgV0") // no print
	oV2.MsgStdOutV1("2msgV1") // no print
	oV2.MsgStdOutV2("2msgV2") // print
	oV2.MsgStdOutV3("2msgV3") // no print

	oV3.MsgStdOutV0("3msgV0") // no print
	oV3.MsgStdOutV1("3msgV1") // no print
	oV3.MsgStdOutV2("3msgV2") // no print
	oV3.MsgStdOutV3("3msgV3") // print

	oV0.MsgStdErr("0err")       // print
	oV0.MsgStdErrV("0errV")     // no print
	oV0.MsgStdErrVV("0errVV")   // no print
	oV0.MsgStdErrVVV("0errVVV") // no print

	oV1.MsgStdErr("1err")       // print
	oV1.MsgStdErrV("1errV")     // print
	oV1.MsgStdErrVV("1errVV")   // no print
	oV1.MsgStdErrVVV("1errVVV") // no print

	oV2.MsgStdErr("2err")       // print
	oV2.MsgStdErrV("2errV")     // print
	oV2.MsgStdErrVV("2errVV")   // print
	oV2.MsgStdErrVVV("2errVVV") // no print

	oV3.MsgStdErr("3err")       // print
	oV3.MsgStdErrV("3errV")     // print
	oV3.MsgStdErrVV("3errVV")   // print
	oV3.MsgStdErrVVV("3errVVV") // print

	oV0.MsgStdErrV0("0errV0") // print
	oV0.MsgStdErrV1("0errV1") // no print
	oV0.MsgStdErrV2("0errV2") // no print
	oV0.MsgStdErrV3("0errV3") // no print

	oV1.MsgStdErrV0("1errV0") // no print
	oV1.MsgStdErrV1("1errV1") // print
	oV1.MsgStdErrV2("1errV2") // no print
	oV1.MsgStdErrV3("1errV3") // no print

	oV2.MsgStdErrV0("2errV0") // no print
	oV2.MsgStdErrV1("2errV1") // no print
	oV2.MsgStdErrV2("2errV2") // print
	oV2.MsgStdErrV3("2errV3") // no print

	oV3.MsgStdErrV0("3errV0") // no print
	oV3.MsgStdErrV1("3errV1") // no print
	oV3.MsgStdErrV2("3errV2") // no print
	oV3.MsgStdErrV3("3errV3") // print

	assert.EqualStrings(t, "0msg\n1msg\n1msgV\n2msg\n2msgV\n2msgVV\n3msg\n3msgV\n3msgVV\n3msgVVV\n0msgV0\n1msgV1\n2msgV2\n3msgV3\n", stdout.String())
	assert.EqualStrings(t, "0err\n1err\n1errV\n2err\n2errV\n2errVV\n3err\n3errV\n3errVV\n3errVVV\n0errV0\n1errV1\n2errV2\n3errV3\n", stderr.String())
}
