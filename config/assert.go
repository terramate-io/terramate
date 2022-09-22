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

package config

import (
	"github.com/mineiros-io/terramate/hcl"
	"github.com/mineiros-io/terramate/hcl/eval"
)

// Assert represents evaluated assert block configuration.
type Assert struct {
	Assertion bool
	Warning   bool
	Message   string
}

// EvalAssert evaluates a given assert configuration and returns its
// evaluated form.
func EvalAssert(evalctx *eval.Context, cfg hcl.AssertConfig) (Assert, error) {
	assertionVal, _ := evalctx.Eval(cfg.Assertion)
	assertion := assertionVal.True()
	messageVal, _ := evalctx.Eval(cfg.Message)
	message := messageVal.AsString()
	return Assert{
		Assertion: assertion,
		Message:   message,
	}, nil
}
