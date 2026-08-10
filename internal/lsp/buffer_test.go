package lsp

import "testing"

func TestBufferStoreGetReturnsSnapshot(t *testing.T) {
	store := NewBufferStore()
	source := &Buffer{URI: "file:///test.go", Text: "original", Version: 1}
	store.Set(source)
	source.Text = "modified source"

	buffer, ok := store.Get("file:///test.go")
	if !ok {
		t.Fatal("buffer not found")
	}
	buffer.Text = "modified"

	snapshot, ok := store.Get("file:///test.go")
	if !ok {
		t.Fatal("buffer not found")
	}
	if snapshot.Text != "original" {
		t.Fatalf("stored text = %q, want %q", snapshot.Text, "original")
	}
}
