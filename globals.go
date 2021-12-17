package terramate

// StackGlobals holds all the globals defined for a stack.
// The zero type is valid and it represents "no globals defined".
type StackGlobals struct {
}

// LoadStackGlobals loads from the file system all globals defined for
// a given stack. It will navigate the file system from stackdir until
// it reaches rootdir, loading globals and merging them appropriately.
//
// More specific globals (closer or at the stack) have precedence over
// less specific globals (closer or at the root dir).
//
// Both rootdir and stackdir must be absolute paths.
func LoadStackGlobals(rootdir string, stackdir string) (StackGlobals, error) {
	return StackGlobals{}, nil
}

// Equal checks if two StackGlobals are equal. They are equal if both
// have globals with the same name=value.
func (sg StackGlobals) Equal(other StackGlobals) bool {
	return true
}
