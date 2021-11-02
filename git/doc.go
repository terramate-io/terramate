// Package git provides a wrapper for the git command line program.
// The helper methods avoids porcelain commands as much as possible and
// return a parsed output whenever possible.
//
// Users of this package have access to the low-level Exec() function for the
// methods not yet implemented.
package git
