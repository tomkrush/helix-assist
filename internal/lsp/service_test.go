package lsp

import (
	"encoding/json"
	"testing"
)

func TestDidChangeIsAppliedSynchronously(t *testing.T) {
	svc := NewService(ServerCapabilities{}, NewLogger(""), "test")
	const uri = "file:///test.go"
	svc.Buffers.Set(&Buffer{URI: uri, Text: "old", Version: 1})

	params, err := json.Marshal(DidChangeParams{
		TextDocument: VersionedTextDocumentIdentifier{
			TextDocumentIdentifier: TextDocumentIdentifier{URI: uri},
			Version:                2,
		},
		ContentChanges: []ContentChange{{Text: "new"}},
	})
	if err != nil {
		t.Fatal(err)
	}

	svc.emit(EventDidChange, &JSONRPCMessage{Params: params})
	buffer, ok := svc.Buffers.Get(uri)
	if !ok {
		t.Fatal("buffer not found")
	}
	if buffer.Text != "new" || buffer.Version != 2 {
		t.Fatalf("buffer = %#v, want text new at version 2", buffer)
	}
}
