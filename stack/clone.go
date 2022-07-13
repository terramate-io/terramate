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

package stack

import (
	"strings"

	"github.com/mineiros-io/terramate/errors"
)

// Clone will clone the stack at srcdir into targetdir.
//
// - srcdir must be a stack (fail otherwise)
// - targetdir must not exist (fail otherwise)
// - All files and directories are copied  (except dotfiles/dirs)
// - If cloned stack has an ID it will be adjusted to a generated UUID.
// - If cloned stack has no ID the cloned stack also won't have an ID.
func Clone(rootdir, targetdir, srcdir string) error {
	if !strings.HasPrefix(srcdir, rootdir) {
		return errors.E(ErrInvalidStackDir, "src dir %q must be inside project root %q", srcdir, rootdir)
	}
	if !strings.HasPrefix(targetdir, rootdir) {
		return errors.E(ErrInvalidStackDir, "target dir %q must be inside project root %q", targetdir, rootdir)
	}
	return nil
}
