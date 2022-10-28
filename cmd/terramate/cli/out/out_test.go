// Copyright 2022 Mineiros GmbH
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package out_test

import (
	"bytes"
	"testing"

	"github.com/madlambda/spells/assert"
	"github.com/mineiros-io/terramate/cmd/terramate/cli/out"
)

func TestOutput(t *testing.T) {
	stdout := bytes.Buffer{}
	stderr := bytes.Buffer{}
	o := out.New(out.V, &stdout, &stderr)

	o.Msg(out.V, "message1")
	o.Msg(out.V, "message%d", 2)
	o.Msg(out.VV, "message3")
	o.Msg(out.VVV, "message4")

	o.Err(out.V, "err1")
	o.Err(out.V, "err%d", 2)
	o.Err(out.VV, "err3")
	o.Err(out.VVV, "err4")

	assert.EqualStrings(t, "message1\nmessage2\n", stdout.String())
	assert.EqualStrings(t, "err1\nerr2\n", stderr.String())
}
