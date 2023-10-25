// Copyright 2023 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package testserver

import "io"

func write(w io.Writer, data []byte) {
	_, _ = w.Write(data)
}

func writeErr(w io.Writer, err error) {
	_, _ = w.Write([]byte(err.Error()))
}

func writeString(w io.Writer, str string) {
	write(w, []byte(str))
}

func justClose(c io.Closer) {
	_ = c.Close()
}
