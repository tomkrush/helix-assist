package handlers

import (
	"testing"

	"github.com/leona/helix-assist/internal/lsp"
	"github.com/leona/helix-assist/internal/util"
)

func TestCompletionLabelIncludesTypedWord(t *testing.T) {
	h := &CompletionHandler{}
	item := h.buildCompletionItem(
		`("Hello, World!")`,
		util.ContentParts{LastLine: "\tfmt.Print"},
		lsp.Position{Line: 3, Character: 10},
	)

	if item.Label != `Print("Hello, World!")` {
		t.Fatalf("label = %q", item.Label)
	}
	if item.TextEdit == nil || item.TextEdit.Range.Start != (lsp.Position{Line: 3, Character: 10}) ||
		item.TextEdit.Range.End != (lsp.Position{Line: 3, Character: 10}) {
		t.Fatalf("textEdit range = %#v, want zero-width edit at cursor", item.TextEdit)
	}
}

func TestCompletionWordPrefix(t *testing.T) {
	for _, test := range []struct {
		line string
		want string
	}{
		{"\tfmt.Print", "Print"},
		{"background-col", "background-col"},
		{"value + ", ""},
		{"object.", ""},
	} {
		if got := completionWordPrefix(test.line); got != test.want {
			t.Errorf("completionWordPrefix(%q) = %q, want %q", test.line, got, test.want)
		}
	}
}

func TestBuildCompletionItemPreservesLeadingSpaceInStringContinuation(t *testing.T) {
	h := &CompletionHandler{}
	content := util.ContentParts{
		ContentBefore: `fmt.Println("hello`,
		LastLine:      `fmt.Println("hello`,
	}

	item := h.buildCompletionItem(" world", content, lsp.Position{Line: 0, Character: 18})

	if item.Label != "hello world" {
		t.Fatalf("Label = %q, want %q", item.Label, "hello world")
	}
	if item.TextEdit == nil || item.TextEdit.NewText != " world" {
		t.Fatalf("TextEdit = %#v, want insertion of %q", item.TextEdit, " world")
	}
}

func TestBuildCompletionItemKeepsSpaceWhenRemovingRepeatedLine(t *testing.T) {
	h := &CompletionHandler{}
	content := util.ContentParts{
		ContentBefore: `fmt.Println("hello`,
		LastLine:      `fmt.Println("hello`,
	}

	item := h.buildCompletionItem(`fmt.Println("hello world")`, content, lsp.Position{Line: 0, Character: 18})

	if item.TextEdit == nil || item.TextEdit.NewText != ` world")` {
		t.Fatalf("TextEdit = %#v, want insertion of %q", item.TextEdit, ` world")`)
	}
}

func TestBuildCompletionItemUsesUTF16LengthForOverlap(t *testing.T) {
	h := &CompletionHandler{}
	position := lsp.Position{Line: 0, Character: 10}
	content := util.ContentParts{ContentImmediatelyAfter: "🙂)"}

	item := h.buildCompletionItem("🙂)", content, position)

	if item.TextEdit == nil || item.TextEdit.Range.End != (lsp.Position{Line: 0, Character: 13}) {
		t.Fatalf("TextEdit = %#v, want three UTF-16 code units replaced", item.TextEdit)
	}
}

func TestBuildCompletionItemReplacesOnlyExistingClosingDelimiter(t *testing.T) {
	h := &CompletionHandler{}
	position := lsp.Position{Line: 2, Character: 20}
	content := util.ContentParts{
		LastLine:                `fmt.Println("hello`,
		ContentImmediatelyAfter: `")`,
	}

	item := h.buildCompletionItem(` world")`, content, position)

	if item.TextEdit == nil {
		t.Fatal("TextEdit is nil")
	}
	if item.TextEdit.Range.Start != position {
		t.Fatalf("TextEdit start = %#v, want %#v", item.TextEdit.Range.Start, position)
	}
	if item.TextEdit.Range.End != (lsp.Position{Line: 2, Character: 22}) {
		t.Fatalf("TextEdit end = %#v", item.TextEdit.Range.End)
	}
	if item.TextEdit.NewText != ` world")` {
		t.Fatalf("TextEdit NewText = %q", item.TextEdit.NewText)
	}
}

func TestBuildCompletionItemAddsMissingSpaceInComment(t *testing.T) {
	h := &CompletionHandler{}
	content := util.ContentParts{
		ContentBefore: `// Connect to the database and`,
		LastLine:      `// Connect to the database and`,
	}

	item := h.buildCompletionItem("run a query", content, lsp.Position{Line: 0, Character: 30})

	if item.TextEdit == nil || item.TextEdit.NewText != " run a query" {
		t.Fatalf("TextEdit = %#v, want comment continuation with a leading space", item.TextEdit)
	}
	if item.Label != "and run a query" {
		t.Fatalf("Label = %q, want %q", item.Label, "and run a query")
	}
}

func TestBuildCompletionItemDoesNotAddSpaceInCode(t *testing.T) {
	h := &CompletionHandler{}
	content := util.ContentParts{ContentBefore: "fmt.Print", LastLine: "fmt.Print"}

	item := h.buildCompletionItem("ln", content, lsp.Position{Line: 0, Character: 9})

	if item.TextEdit == nil || item.TextEdit.NewText != "ln" {
		t.Fatalf("TextEdit = %#v, want code suffix without a leading space", item.TextEdit)
	}
}
