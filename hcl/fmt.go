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

package hcl

import "github.com/hashicorp/hcl/v2/hclwrite"

// Format will format the given hcl.
func Format(hcl string) string {
	// For now we just use plain hclwrite.Format
	// but we plan on customizing formatting in the near future.
	return string(hclwrite.Format([]byte(hcl)))
}
