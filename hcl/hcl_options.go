// Copyright 2025 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package hcl

// WithExperiments is an option to set the experiments to be enabled in the parser.
func WithExperiments(experiments ...string) Option {
	return func(p *TerramateParser) {
		p.experiments = experiments
	}
}

// WithStrictMode is an option to enable strict mode in the parser.
func WithStrictMode() Option {
	return func(p *TerramateParser) {
		p.strict = true
	}
}

// WithUnmergedBlockHandlers is an option to set the unmerged block specifications for the parser.
func WithUnmergedBlockHandlers(specs ...UnmergedBlockHandlerConstructor) Option {
	return func(p *TerramateParser) {
		for _, spec := range specs {
			p.addUnmergedBlockHandler(spec)
		}
	}
}

// WithMergedBlockHandlers is an option to set the merged block specifications for the parser.
func WithMergedBlockHandlers(specs ...MergedBlockHandlerConstructor) Option {
	return func(p *TerramateParser) {
		for _, spec := range specs {
			p.addMergedBlockHandler(spec)
		}
	}
}

// WithUniqueBlockHandlers is an option to set the unique block specifications for the parser.
func WithUniqueBlockHandlers(specs ...UniqueBlockHandlerConstructor) Option {
	return func(p *TerramateParser) {
		for _, spec := range specs {
			p.addUniqueBlockHandler(spec)
		}
	}
}

// WithMergedLabelsBlockHandlers is an option to set the merged labels block specifications for the parser.
func WithMergedLabelsBlockHandlers(specs ...MergedLabelsBlockHandlerConstructor) Option {
	return func(p *TerramateParser) {
		for _, spec := range specs {
			p.addMergedLabelsBlockHandler(spec)
		}
	}
}
