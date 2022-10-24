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

package hcl_test

import (
	"path/filepath"
	"testing"

	"github.com/madlambda/spells/assert"
	"github.com/mineiros-io/terramate/hcl"
	. "github.com/mineiros-io/terramate/test/hclutils"
)

func TestRangeFromHCLRange(t *testing.T) {

	rootdir := "/host/path/root"
	path := "/dir/sub/assert.tm"
	start := Start(1, 1, 0)
	end := End(3, 2, 37)
	hclrange := Mkrange(filepath.Join(rootdir, path), start, end)
	tmrange := hcl.NewRange(rootdir, hclrange)

	assert.EqualStrings(t, hclrange.Filename, tmrange.Filename())

	assert.EqualInts(t, hclrange.Start.Line, tmrange.Start().Line())
	assert.EqualInts(t, hclrange.Start.Column, tmrange.Start().Column())
	assert.EqualInts(t, hclrange.Start.Byte, tmrange.Start().Byte())

	assert.EqualInts(t, hclrange.End.Line, tmrange.End().Line())
	assert.EqualInts(t, hclrange.End.Column, tmrange.End().Column())
	assert.EqualInts(t, hclrange.End.Byte, tmrange.End().Byte())
}
