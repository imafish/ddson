package logging

import (
	"context"
	"log/slog"
	"testing"
)

func TestDefaultLogger(t *testing.T) {
	l := DefaultLogger()
	if l == nil {
		t.Fatalf("expected non-nil logger")
	}
	l.Info("test")
}

func TestNewSlogLoggerNil(t *testing.T) {
	l := NewSlogLogger(nil)
	if l == nil {
		t.Fatalf("expected non-nil logger")
	}
	l.Log(context.Background(), slog.LevelInfo, "test")
}
