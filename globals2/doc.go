// Package globals2 implements the Terramate globals feature.
//
// Globals can be defined in any `.tm` file using a `globals` block, like below:
//
//	globals {
//	  name = "Terramate"
//	  year = 2023
//	}
//
// The concept is similar to declaring variables in general purpose programming
// languages but once they are evaluated they cannot be updated anymore.
// The globals are actually scoped variables and its lifetime is controlled by
// the directory where it's defined.
//
// Each directory defines a new globals scope and the globals defined in the
// directory are inherited by child directories (child scopes).
// Then, in the Terramate terminology, directories are variable scopes.
// As in most programming languages, child scopes can shadow the globals from
// parent scopes by declaring a variable with same name.
package globals2
