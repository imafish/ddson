package downloadtask

import (
	"context"
	"log/slog"
	"testing"

	"github.com/imafish/ddson/internal/logging"
)

type noopLogger struct{}

func (noopLogger) Debug(string, ...any)                                    {}
func (noopLogger) Info(string, ...any)                                     {}
func (noopLogger) Warn(string, ...any)                                     {}
func (noopLogger) Error(string, ...any)                                    {}
func (noopLogger) Log(_ context.Context, _ slog.Level, _ string, _ ...any) {}

func TestSetLogger(t *testing.T) {
	SetLogger(logging.DefaultLogger())
	SetLogger(noopLogger{})
}
