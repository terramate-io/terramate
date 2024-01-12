// Copyright 2024 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package git

// Options is a customizable options object to be used with the [Git]
// wrapper.
type Options struct {
	config Config
}

// WorkingDir sets the wrapper working directory.
func (opt *Options) WorkingDir(wd string) *Options {
	opt.config.WorkingDir = wd
	return opt
}

// Wrapper returns a new wrapper with the given options.
func (opt Options) Wrapper() *Git {
	return &Git{
		options: opt,
	}
}
