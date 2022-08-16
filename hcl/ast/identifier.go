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

package ast

import (
	"github.com/smasher164/xid"
)

// IsIdentifier checks if the given string is a valid HCL identifier.
func IsIdentifier(input string) bool {
	if input == "" {
		return false
	}

	for i, r := range input {
		if i == 0 {
			if !xid.Start(r) {
				return false
			}
		} else {
			// The '-' is explicitly allowed by HCL spec.
			if !xid.Continue(r) && r != rune('-') {
				return false
			}
		}
	}
	return true
}
