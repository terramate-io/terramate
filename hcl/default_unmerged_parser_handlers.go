// Copyright 2025 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package hcl

// UnmergedBlockHandlerConstructor is a constructor for an unmerged block handler.
type UnmergedBlockHandlerConstructor func() UnmergedBlockHandler

// DefaultUnmergedBlockParsers returns the default unmerged block specifications for the parser.
func DefaultUnmergedBlockParsers() []UnmergedBlockHandlerConstructor {
	return []UnmergedBlockHandlerConstructor{
		newStackBlockConstructor,
		newTopLevelAssertBlockConstructor,
		newGenerateHCLBlockConstructor,
		newGenerateFileBlockConstructor,
		newSharingBackendBlockConstructor,
		newInputBlockConstructor,
		newOutputBlockConstructor,
		newScriptBlockConstructor,
		newScaffoldBlockConstructor,
		newEnvironmentBlockConstructor,
	}
}

func newStackBlockConstructor() UnmergedBlockHandler {
	return NewStackBlockParser()
}

func newTopLevelAssertBlockConstructor() UnmergedBlockHandler {
	return NewTopLevelAssertBlockParser()
}

func newGenerateHCLBlockConstructor() UnmergedBlockHandler {
	return NewGenerateHCLBlockParser()
}

func newGenerateFileBlockConstructor() UnmergedBlockHandler {
	return NewGenerateFileBlockParser()
}

func newSharingBackendBlockConstructor() UnmergedBlockHandler {
	return NewSharingBackendBlockParser()
}

func newInputBlockConstructor() UnmergedBlockHandler {
	return NewInputBlockParser()
}

func newOutputBlockConstructor() UnmergedBlockHandler {
	return NewOutputBlockParser()
}

func newScriptBlockConstructor() UnmergedBlockHandler {
	return NewScriptBlockParser()
}

func newScaffoldBlockConstructor() UnmergedBlockHandler {
	return NewScaffoldBlockParser()
}

func newEnvironmentBlockConstructor() UnmergedBlockHandler {
	return NewEnvironmentBlockParser()
}
