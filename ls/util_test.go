// Copyright 2025 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package tmls

import (
	"testing"
)

func TestPosToByteOffset(t *testing.T) {
	tests := []struct {
		name       string
		content    string
		line       int
		character  int
		wantOffset int
	}{
		{
			name:       "ASCII only",
			content:    "hello world",
			line:       0,
			character:  6,
			wantOffset: 6, // position after "hello "
		},
		{
			name:       "café - 2-byte UTF-8 character (é)",
			content:    "café",
			line:       0,
			character:  3, // position after "caf"
			wantOffset: 3,
		},
		{
			name:       "café - position after é",
			content:    "café",
			line:       0,
			character:  4, // position after "café"
			wantOffset: 5, // 'c'(1) + 'a'(1) + 'f'(1) + 'é'(2) = 5 bytes
		},
		{
			name:       "emoji - 4-byte UTF-8 character (😀)",
			content:    "hello😀world",
			line:       0,
			character:  5, // position before emoji
			wantOffset: 5,
		},
		{
			name:       "emoji - position after emoji",
			content:    "hello😀world",
			line:       0,
			character:  7, // position after emoji (emoji counts as 2 UTF-16 code units)
			wantOffset: 9, // "hello"(5) + "😀"(4) = 9 bytes
		},
		{
			name:       "multiple lines",
			content:    "line1\nline2",
			line:       1,
			character:  3,
			wantOffset: 9, // "line1\n"(6) + "lin"(3) = 9
		},
		{
			name:       "multiple lines with UTF-8",
			content:    "café\nworld",
			line:       1,
			character:  2,
			wantOffset: 8, // "café"(5 bytes: c+a+f+é(2bytes)) + "\n"(1) + "wo"(2) = 8
		},
		{
			name:       "Chinese characters - 3-byte UTF-8",
			content:    "你好世界",
			line:       0,
			character:  2, // position after "你好"
			wantOffset: 6, // Each Chinese char is 3 bytes in UTF-8: 3 + 3 = 6
		},
		{
			name:       "mixed ASCII and multi-byte",
			content:    "test 测试 data",
			line:       0,
			character:  7,  // position after "test 测试"
			wantOffset: 11, // "test "(5) + "测试"(6) = 11
		},
		{
			name:       "end of content",
			content:    "test",
			line:       0,
			character:  4,
			wantOffset: 4,
		},
		{
			name:       "beyond end of content",
			content:    "test",
			line:       0,
			character:  10,
			wantOffset: 4, // should return len(content)
		},
		{
			name:       "emoji at start",
			content:    "🎉hello",
			line:       0,
			character:  2, // after emoji (counts as 2 UTF-16 code units)
			wantOffset: 4, // emoji is 4 bytes in UTF-8
		},
		{
			name:       "combining characters",
			content:    "e\u0301", // é as 'e' + combining acute accent
			line:       0,
			character:  2, // after both characters
			wantOffset: 3, // 'e'(1) + combining accent(2) = 3 bytes
		},
		{
			name:       "verify UTF-16 not byte counting - café after é",
			content:    "café",
			line:       0,
			character:  4, // position after "café" (LSP counts UTF-16 code units, not bytes)
			wantOffset: 5, // Must be byte 5, not byte 4 (byte 4 is middle of multi-byte é)
			// This test explicitly verifies we don't count bytes but UTF-16 code units
			// If we counted bytes: 'c'(1) + 'a'(1) + 'f'(1) + 'é'(1 byte counted) = 4 (WRONG)
			// Correct: 'c'(1) + 'a'(1) + 'f'(1) + 'é'(2 bytes) = 5 bytes, but character position 4 because é is 1 UTF-16 code unit
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotOffset := posToByteOffset([]byte(tt.content), tt.line, tt.character)
			if gotOffset != tt.wantOffset {
				t.Errorf("posToByteOffset() = %d, want %d", gotOffset, tt.wantOffset)
				t.Logf("Content: %q", tt.content)
				t.Logf("Content bytes: %v", []byte(tt.content))
				t.Logf("Looking for line=%d, character=%d", tt.line, tt.character)
			}
		})
	}
}
