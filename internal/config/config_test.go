package config

import "testing"

func TestDefaultTriggersIncludeMemberAccess(t *testing.T) {
	for _, trigger := range DefaultConfig().TriggerCharacters {
		if trigger == "." {
			return
		}
	}
	t.Fatal("default trigger characters do not include dot")
}
