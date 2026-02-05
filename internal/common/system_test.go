package common

import "testing"

func TestOriginalUserHomeDir(t *testing.T) {
	home, err := OriginalUserHomeDir()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if home == "" {
		t.Fatalf("expected non-empty home dir")
	}
}
