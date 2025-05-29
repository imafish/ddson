package logging

import (
	"context"
	"fmt"
	"io"
	"log/slog"
)

// Colors for terminal output
const (
	colorReset   = "\033[0m"
	colorRed     = "\033[31m"
	colorGreen   = "\033[32m"
	colorYellow  = "\033[33m"
	colorBlue    = "\033[34m"
	colorMagenta = "\033[35m"
	colorCyan    = "\033[36m"
	colorWhite   = "\033[37m"
	colorBold    = "\033[1m"
	colorDim     = "\033[2m"
)

type colors struct {
	Reset           string
	Red             string
	Green           string
	Yellow          string
	Blue            string
	Magenta         string
	Cyan            string
	White           string
	Bold            string
	Dim             string
	DimmedEqualMark string
}

var defaultColors = colors{
	Reset:           colorReset,
	Red:             colorRed,
	Green:           colorGreen,
	Yellow:          colorYellow,
	Blue:            colorBlue,
	Magenta:         colorMagenta,
	Cyan:            colorCyan,
	White:           colorWhite,
	Bold:            colorBold,
	Dim:             colorDim,
	DimmedEqualMark: "\033[2m=\033[0m",
}

var nocolorColors = colors{
	Reset:           "",
	Red:             "",
	Green:           "",
	Yellow:          "",
	Blue:            "",
	Magenta:         "",
	Cyan:            "",
	White:           "",
	Bold:            "",
	Dim:             "",
	DimmedEqualMark: "=",
}

// Custom handler that implements slog.Handler
type CustomHandler struct {
	handler slog.Handler
	output  io.Writer
	colors  *colors
}

func (h *CustomHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.handler.Enabled(ctx, level)
}

func (h *CustomHandler) Handle(ctx context.Context, r slog.Record) error {
	// Format timestamp (brief format)
	timeStr := r.Time.Format("060102-15:04:05.000")

	// Format level (4-letter width)
	levelStr := r.Level.String()
	switch {
	case len(levelStr) > 5:
		levelStr = levelStr[:5]
	case len(levelStr) < 5:
		levelStr = fmt.Sprintf("%-5s", levelStr)
	}

	// Get level color
	var levelColor string
	switch r.Level {
	case slog.LevelDebug:
		levelColor = h.colors.Blue
	case slog.LevelInfo:
		levelColor = h.colors.Green
	case slog.LevelWarn:
		levelColor = h.colors.Yellow
	case slog.LevelError:
		levelColor = h.colors.Red
	default:
		levelColor = h.colors.White
	}

	// Write the main log line
	fmt.Fprintf(h.output, "%s %s%-5s%s %s",
		h.colors.Dim+timeStr+h.colors.Reset,
		levelColor, levelStr, h.colors.Reset,
		r.Message)

	// Add attributes with colored keys
	r.Attrs(func(attr slog.Attr) bool {
		fmt.Fprintf(h.output, " %s%s%s%s%v",
			h.colors.Magenta, attr.Key, h.colors.Reset, h.colors.DimmedEqualMark,
			attr.Value.Any())
		return true
	})

	fmt.Fprintln(h.output) // Newline at end
	return nil
}

func (h *CustomHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &CustomHandler{
		handler: h.handler.WithAttrs(attrs),
		output:  h.output,
	}
}

func (h *CustomHandler) WithGroup(name string) slog.Handler {
	return &CustomHandler{
		handler: h.handler.WithGroup(name),
		output:  h.output,
	}
}

// NewCustomLogger creates a new logger with our custom format
func NewCustomLogger(w io.Writer, level slog.Level, isColorful bool) *slog.Logger {
	var colorsToUse *colors
	if isColorful {
		colorsToUse = &defaultColors
	} else {
		colorsToUse = &nocolorColors
	}

	handler := &CustomHandler{
		handler: slog.NewTextHandler(w, &slog.HandlerOptions{
			Level: level,
		}),
		output: w,
		colors: colorsToUse,
	}
	return slog.New(handler)
}
