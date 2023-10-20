// Copyright 2023 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package cloud

import (
	"bytes"
	"strings"
	"testing"
)

const nlines = 10000

func BenchmarkCloudReadLines(b *testing.B) {
	b.ReportAllocs()
	b.StopTimer()
	var bufData string
	for i := 0; i < nlines; i++ {
		bufData += strings.Repeat("A", 80) + "\n"
	}
	b.StartTimer()
	for i := 0; i < b.N; i++ {
		buf := bytes.NewBufferString(bufData)
		var pending []byte
		for {
			_, rest, err := readLines(buf, pending[:])
			if err != nil {
				break
			}
			pending = rest
		}
	}
}

func BenchmarkCloudReadLine(b *testing.B) {
	b.ReportAllocs()
	b.StopTimer()
	var bufData string
	for i := 0; i < nlines; i++ {
		bufData += strings.Repeat("A", 80) + "\n"
	}
	b.StartTimer()
	for i := 0; i < b.N; i++ {
		buf := bytes.NewBufferString(bufData)
		for {
			_, err := readLine(buf)
			if err != nil {
				break
			}
		}
	}
}
