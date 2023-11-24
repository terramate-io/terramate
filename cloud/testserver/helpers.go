// Copyright 2023 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package testserver

import (
	"encoding/json"
	"io"
	"log"
)

func write(w io.Writer, data []byte) {
	_, err := w.Write(data)
	if err != nil {
		panic(err)
	}
}

func writeErr(w io.Writer, err error) {
	write(w, []byte(err.Error()))
}

func writeString(w io.Writer, str string) {
	write(w, []byte(str))
}

func justClose(c io.Closer) {
	if err := c.Close(); err != nil {
		log.Printf("error: %v", err)
	}
}

func marshalWrite(w io.Writer, obj interface{}) {
	data, err := json.Marshal(obj)
	if err != nil {
		panic(err)
	}
	write(w, data)
}
