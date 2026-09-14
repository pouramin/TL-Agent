package main

import (
	"os"
	"strconv"
	"testing"
)

func TestKiloRuntimeDefaults(t *testing.T) {
	for key, want := range map[string]string{
		"KILO_TELEMETRY_LEVEL":    "off",
		"DO_NOT_TRACK":             "1",
		"OTEL_SDK_DISABLED":        "true",
		"KILO_ENABLE_QUESTION_TOOL": "true",
	} {
		if got := os.Getenv(key); got != want {
			t.Fatalf("%s=%q, want %q", key, got, want)
		}
	}

	if got, want := os.Getenv("KILO_PARENT_PID"), strconv.Itoa(os.Getpid()); got != want {
		t.Fatalf("KILO_PARENT_PID=%q, want %q", got, want)
	}
	if got := os.Getenv("KILO_CONFIG_CONTENT"); got == "" {
		t.Fatal("KILO_CONFIG_CONTENT should provide a safe edit permission default")
	}
}
