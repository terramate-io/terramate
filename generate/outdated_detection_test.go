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

package generate_test

// Tests Inside stacks
// - Empty project
// - Has generate but code is not generated yet
// - Has generate but code is outdated
// - Has no generate but code is present
// - Blocks with same label but different conditions (true, false, false) (false, true, false) (false, false, true)
// - Block with condition false and old code is present
// - Block with condition false and no code is present

// Tests Outside Stacks
// - Generated files outside stacks are detected
