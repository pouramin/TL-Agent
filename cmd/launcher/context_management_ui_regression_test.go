package main

import (
	"strings"
	"testing"
)

func TestContextManagementUIRegressions(t *testing.T) {
	source := readBrowserSource(t, "presentation.ts")
	for _, required := range []string{
		`item?.kind === "context"`,
		`Context window managed`,
		`Omitted persisted messages`,
		`Compacted tool results`,
		`TL Studio context checkpoint`,
	} {
		if !strings.Contains(source, required) {
			t.Fatalf("context management presentation missing %q", required)
		}
	}
}
