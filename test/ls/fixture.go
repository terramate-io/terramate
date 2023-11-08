// Copyright 2023 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package ls

import (
	"context"
	"io"
	"net"
	"testing"

	tmls "github.com/terramate-io/terramate/ls"
	"github.com/terramate-io/terramate/test/sandbox"
	"go.lsp.dev/jsonrpc2"
)

// Fixture is the default test fixture.
type Fixture struct {
	Sandbox sandbox.S
	Editor  *Editor
}

// Setup a new fixture.
func Setup(t *testing.T, layout ...string) Fixture {
	t.Helper()

	s := sandbox.NoGit(t, true)
	s.BuildTree(layout)

	// WHY: LSP is bidirectional, the editor calls the server
	// and the server also calls the editor (not only sending responses),
	// It is not a classic request/response protocol so we need both
	// running + connected through a pipe.

	editorRW, serverRW := net.Pipe()

	serverConn := jsonrpc2Conn(serverRW)
	server := tmls.NewServer(serverConn)
	serverConn.Go(context.Background(), server.Handler)

	editorConn := jsonrpc2Conn(editorRW)
	e := NewEditor(t, s, editorConn)
	editorConn.Go(context.Background(), e.Handler)

	t.Cleanup(func() {
		if err := editorConn.Close(); err != nil {
			t.Errorf("closing editor connection: %v", err)
		}
		if err := serverConn.Close(); err != nil {
			t.Errorf("closing server connection: %v", err)
		}

		<-editorConn.Done()
		<-serverConn.Done()

		// Now that we closed and waited for the editor to stop
		// we can check that no requests were left unhandled by the test
		select {
		case req := <-e.Requests:
			{
				t.Fatalf("unhandled editor request: %s %s", req.Method(), req.Params())
			}
		default:
		}
	})

	return Fixture{
		Editor:  e,
		Sandbox: s,
	}
}

func jsonrpc2Conn(rw io.ReadWriteCloser) jsonrpc2.Conn {
	stream := jsonrpc2.NewStream(rw)
	return jsonrpc2.NewConn(stream)
}
