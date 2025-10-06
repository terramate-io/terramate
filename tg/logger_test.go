// Copyright 2025 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package tg

import (
	"testing"
)

func TestFormatArgs(t *testing.T) {
	tests := []struct {
		name     string
		args     []interface{}
		expected string
	}{
		{
			name:     "empty args",
			args:     []interface{}{},
			expected: "",
		},
		{
			name:     "single string",
			args:     []interface{}{"test message"},
			expected: "test message",
		},
		{
			name:     "single int",
			args:     []interface{}{42},
			expected: "42",
		},
		{
			name:     "single bool",
			args:     []interface{}{true},
			expected: "true",
		},
		{
			name:     "multiple strings",
			args:     []interface{}{"hello", "world"},
			expected: "helloworld",
		},
		{
			name:     "multiple strings with space",
			args:     []interface{}{"hello ", "world"},
			expected: "hello world",
		},
		{
			name:     "mixed types",
			args:     []interface{}{"count:", 42, "active:", true},
			expected: "count:42active:true",
		},
		{
			name:     "mixed types with spaces",
			args:     []interface{}{"count: ", 42, " active: ", true},
			expected: "count: 42 active: true",
		},
		{
			name:     "string and error",
			args:     []interface{}{"error occurred:", &testError{msg: "test error"}},
			expected: "error occurred:test error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatArgs(tt.args...)
			if result != tt.expected {
				t.Errorf("formatArgs() = %q, want %q", result, tt.expected)
			}
		})
	}
}

type testError struct {
	msg string
}

func (e *testError) Error() string {
	return e.msg
}
