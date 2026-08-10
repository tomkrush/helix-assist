package lsp

import (
	"encoding/json"
	"testing"
)

func TestExplicitNullResultIsPreserved(t *testing.T) {
	id := 7
	data, err := json.Marshal(JSONRPCMessage{
		JSONRPC: "2.0",
		ID:      &id,
		Result:  json.RawMessage("null"),
	})
	if err != nil {
		t.Fatal(err)
	}

	var message map[string]any
	if err := json.Unmarshal(data, &message); err != nil {
		t.Fatal(err)
	}
	result, exists := message["result"]
	if !exists {
		t.Fatalf("response omitted result: %s", data)
	}
	if result != nil {
		t.Fatalf("result = %#v, want nil", result)
	}
}
