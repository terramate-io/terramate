// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

// Package git provides a wrapper for the git command line program.
// The helper methods avoids porcelain commands as much as possible and
// return a parsed output whenever possible.
//
// Users of this package have access to the low-level Exec() function for the
// methods not yet implemented.
package git
