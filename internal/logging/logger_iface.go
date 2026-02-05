package logging

import (
	"context"
	"log/slog"
)

type Logger interface {
	Debug(msg string, args ...any)
	Info(msg string, args ...any)
	Warn(msg string, args ...any)
	Error(msg string, args ...any)
	Log(ctx context.Context, level slog.Level, msg string, args ...any)
}

type SlogLogger struct {
	l *slog.Logger
}

func NewSlogLogger(l *slog.Logger) Logger {
	if l == nil {
		l = slog.Default()
	}
	return &SlogLogger{l: l}
}

func DefaultLogger() Logger {
	return NewSlogLogger(slog.Default())
}

func (s *SlogLogger) Debug(msg string, args ...any) {
	s.l.Debug(msg, args...)
}

func (s *SlogLogger) Info(msg string, args ...any) {
	s.l.Info(msg, args...)
}

func (s *SlogLogger) Warn(msg string, args ...any) {
	s.l.Warn(msg, args...)
}

func (s *SlogLogger) Error(msg string, args ...any) {
	s.l.Error(msg, args...)
}

func (s *SlogLogger) Log(ctx context.Context, level slog.Level, msg string, args ...any) {
	s.l.Log(ctx, level, msg, args...)
}
