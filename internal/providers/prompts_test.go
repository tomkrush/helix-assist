package providers

import (
	"strings"
	"testing"
)

func TestCompletionPromptSupportsStringsAndComments(t *testing.T) {
	prompt := BuildCompletionSystemPrompt("go")

	for _, required := range []string{
		"inside a string literal or comment",
		"short phrase inside strings and comments",
		"do not repeat the opening quote or comment marker",
		"leading space",
	} {
		if !strings.Contains(prompt, required) {
			t.Errorf("completion prompt does not contain %q", required)
		}
	}

	if strings.Contains(prompt, "Do NOT include comments") {
		t.Fatal("completion prompt still unconditionally forbids comment completions")
	}
}
