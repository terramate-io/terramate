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

// Package globals provides functions for loading globals.
//
// The Load loads all the globals visible in the directory, which it means it
// also loads the globals from parent directories.
// The PartialLoad loads all the globals which are fully defined for the
// the directory, which means it doesn't fail if some globals cannot be evaluated
// due to unknows.
//
// Both functions returns a Report containing all globals that did evaluate
// successfully, the pending globals (in the case of partial load) and
// error if any is found.
//
// The usage could be like:
//   report, err := globals.Load(dir)
//   if err != nil {
//       // handle errors
//   }
//   for name, value := range report.Globals {
//       // do something with evaluated global
//   }
//
//   for name, expr := range report.Pending {
//       // do something with pending global
//   }
//
// Both functions modify the context, adding the evaluated globals.
package globals
