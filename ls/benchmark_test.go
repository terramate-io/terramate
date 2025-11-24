// Copyright 2025 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package tmls

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"

	"github.com/terramate-io/terramate/test"
	"github.com/terramate-io/terramate/test/sandbox"
)

// BenchmarkFindDefinition benchmarks the go-to-definition operation
func BenchmarkFindDefinition(b *testing.B) {
	benchmarks := []struct {
		name      string
		fileCount int
	}{
		{"10_files", 10},
		{"50_files", 50},
		{"100_files", 100},
		{"500_files", 500},
	}

	for _, bm := range benchmarks {
		b.Run(bm.name, func(b *testing.B) {
			s := sandbox.New(b)

			// Create a workspace with the specified number of files
			layout := []string{
				`f:globals.tm:globals {
  target_var = "value"
}
`,
			}
			for i := 0; i < bm.fileCount; i++ {
				layout = append(layout, fmt.Sprintf(
					`f:stack%d.tm:stack { name = "stack%d" }`, i, i))
			}
			// Add a file that references the target variable
			layout = append(layout, `f:usage.tm:stack { name = global.target_var }`)

			s.BuildTree(layout)
			srv := newTestServer(b, s.RootDir())

			fname := filepath.Join(s.RootDir(), "usage.tm")
			content := test.ReadFile(b, s.RootDir(), "usage.tm")

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_, err := srv.findDefinitions(fname, []byte(content), 0, 22)
				if err != nil {
					b.Fatalf("unexpected error: %v", err)
				}
			}
		})
	}
}

// BenchmarkFindReferences benchmarks the find-all-references operation
func BenchmarkFindReferences(b *testing.B) {
	benchmarks := []struct {
		name      string
		fileCount int
		refCount  int
	}{
		{"10_files_5_refs", 10, 5},
		{"50_files_10_refs", 50, 10},
		{"100_files_20_refs", 100, 20},
		{"500_files_50_refs", 500, 50},
	}

	for _, bm := range benchmarks {
		b.Run(bm.name, func(b *testing.B) {
			s := sandbox.New(b)

			// Create a workspace with the specified number of files
			layout := []string{
				`f:globals.tm:globals {
  target_var = "value"
}
`,
			}
			// Add files with references
			for i := 0; i < bm.refCount; i++ {
				layout = append(layout, fmt.Sprintf(
					`f:ref%d.tm:stack { name = global.target_var }`, i))
			}
			// Add files without references (noise)
			for i := 0; i < bm.fileCount-bm.refCount; i++ {
				layout = append(layout, fmt.Sprintf(
					`f:stack%d.tm:stack { name = "stack%d" }`, i, i))
			}

			s.BuildTree(layout)
			srv := newTestServer(b, s.RootDir())

			fname := filepath.Join(s.RootDir(), "globals.tm")
			content := test.ReadFile(b, s.RootDir(), "globals.tm")

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_, err := srv.findAllReferences(context.Background(), fname, []byte(content), 1, 2, true)
				if err != nil {
					b.Fatalf("unexpected error: %v", err)
				}
			}
		})
	}
}

// BenchmarkRename benchmarks the rename operation
func BenchmarkRename(b *testing.B) {
	benchmarks := []struct {
		name      string
		fileCount int
		refCount  int
	}{
		{"10_files_5_refs", 10, 5},
		{"50_files_10_refs", 50, 10},
		{"100_files_20_refs", 100, 20},
	}

	for _, bm := range benchmarks {
		b.Run(bm.name, func(b *testing.B) {
			s := sandbox.New(b)

			// Create a workspace with the specified number of files
			layout := []string{
				`f:globals.tm:globals {
  target_var = "value"
}
`,
			}
			// Add files with references
			for i := 0; i < bm.refCount; i++ {
				layout = append(layout, fmt.Sprintf(
					`f:ref%d.tm:stack { name = global.target_var }`, i))
			}
			// Add files without references (noise)
			for i := 0; i < bm.fileCount-bm.refCount; i++ {
				layout = append(layout, fmt.Sprintf(
					`f:stack%d.tm:stack { name = "stack%d" }`, i, i))
			}

			s.BuildTree(layout)
			srv := newTestServer(b, s.RootDir())

			fname := filepath.Join(s.RootDir(), "globals.tm")
			content := test.ReadFile(b, s.RootDir(), "globals.tm")

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_, err := srv.createRenameEdits(context.Background(), fname, []byte(content), 1, 2, "new_name")
				if err != nil {
					b.Fatalf("unexpected error: %v", err)
				}
			}
		})
	}
}

// BenchmarkSearchWorkspace benchmarks the workspace traversal operation
func BenchmarkSearchWorkspace(b *testing.B) {
	benchmarks := []struct {
		name      string
		fileCount int
		depth     int
	}{
		{"flat_100_files", 100, 1},
		{"flat_500_files", 500, 1},
		{"nested_100_files_5_deep", 100, 5},
		{"nested_500_files_5_deep", 500, 5},
	}

	for _, bm := range benchmarks {
		b.Run(bm.name, func(b *testing.B) {
			s := sandbox.New(b)

			layout := []string{
				`f:globals.tm:globals {
  search_var = "value"
}
`,
			}

			// Create files at different depths
			filesPerLevel := bm.fileCount / bm.depth
			for level := 0; level < bm.depth; level++ {
				path := ""
				for d := 0; d <= level; d++ {
					if d > 0 {
						path += "/"
					}
					path += fmt.Sprintf("level%d", d)
				}

				for i := 0; i < filesPerLevel; i++ {
					filename := fmt.Sprintf("file%d.tm", i)
					if path != "" {
						filename = path + "/" + filename
					}
					layout = append(layout, fmt.Sprintf(
						`f:%s:stack { name = "stack" }`, filename))
				}
			}

			s.BuildTree(layout)
			srv := newTestServer(b, s.RootDir())

			info := &symbolInfo{
				namespace:     "global",
				attributeName: "search_var",
				fullPath:      "global.search_var",
			}

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_ = srv.searchReferencesInWorkspace(b.Context(), filepath.Join(s.RootDir(), "globals.tm"), info)
			}
		})
	}
}

// BenchmarkParseFile benchmarks HCL file parsing
func BenchmarkParseFile(b *testing.B) {
	benchmarks := []struct {
		name       string
		lineCount  int
		blockCount int
	}{
		{"small_file_10_lines", 10, 2},
		{"medium_file_50_lines", 50, 5},
		{"large_file_200_lines", 200, 20},
		{"huge_file_1000_lines", 1000, 50},
	}

	for _, bm := range benchmarks {
		b.Run(bm.name, func(b *testing.B) {
			s := sandbox.New(b)

			// Generate a large file with many globals
			content := "globals {\n"
			attrsPerBlock := bm.lineCount / bm.blockCount
			for i := 0; i < attrsPerBlock; i++ {
				content += fmt.Sprintf("  var%d = \"value%d\"\n", i, i)
			}
			content += "}\n\n"

			for block := 1; block < bm.blockCount; block++ {
				content += "globals {\n"
				for i := 0; i < attrsPerBlock; i++ {
					idx := block*attrsPerBlock + i
					content += fmt.Sprintf("  var%d = \"value%d\"\n", idx, idx)
				}
				content += "}\n\n"
			}

			s.BuildTree([]string{
				`f:large.tm:` + content,
			})
			srv := newTestServer(b, s.RootDir())

			fname := filepath.Join(s.RootDir(), "large.tm")

			info := &symbolInfo{
				namespace:     "global",
				attributeName: "var0",
				fullPath:      "global.var0",
			}

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_ = srv.findReferencesInFile(fname, info)
			}
		})
	}
}
