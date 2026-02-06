package progressbar

import (
	"fmt"
	"strings"
	"testing"
)

func TestGenerateOffset(t *testing.T) {
	tests := []struct {
		screenWidth int
		x           int
		progress    float64
		expected    int
	}{
		// Normal cases
		{screenWidth: 80, x: 7, progress: 0.0, expected: 1},
		{screenWidth: 80, x: 7, progress: 0.5, expected: 36},
		{screenWidth: 80, x: 7, progress: 1.0, expected: 72},

		// Edge: progress at boundaries
		{screenWidth: 80, x: 7, progress: -0.1, expected: 0},
		{screenWidth: 80, x: 7, progress: 1.5, expected: 72},

		// Edge: very small screen
		{screenWidth: 5, x: 7, progress: 0.5, expected: 0},
		{screenWidth: 0, x: 7, progress: 0.5, expected: 0},

		// Edge: x equals screenWidth
		{screenWidth: 7, x: 7, progress: 0.5, expected: 0},

		// Edge: x is 0
		{screenWidth: 80, x: 0, progress: 0.5, expected: 40},
	}

	for _, tt := range tests {
		name := fmt.Sprintf("w=%d_x=%d_p=%.1f", tt.screenWidth, tt.x, tt.progress)
		t.Run(name, func(t *testing.T) {
			got := generateOffset(tt.screenWidth, tt.x, tt.progress)
			if got != tt.expected {
				t.Errorf("generateOffset(%d, %d, %f) = %d, want %d",
					tt.screenWidth, tt.x, tt.progress, got, tt.expected)
			}
		})
	}
}

func TestGenerateProgressLine(t *testing.T) {
	tests := []struct {
		name        string
		screenWidth int
		x           int
		progress    float64
		wantLen     int
		wantPrefix  string
		wantSuffix  string
		wantCaret   bool
	}{
		{
			name:        "zero progress",
			screenWidth: 20, x: 4, progress: 0.0,
			wantLen: 17, wantPrefix: "[", wantSuffix: "]", wantCaret: true,
		},
		{
			name:        "half progress",
			screenWidth: 20, x: 4, progress: 0.5,
			wantLen: 17, wantPrefix: "[", wantSuffix: "]", wantCaret: true,
		},
		{
			name:        "full progress",
			screenWidth: 20, x: 4, progress: 1.0,
			wantLen: 17, wantPrefix: "[", wantSuffix: "]", wantCaret: true,
		},
		{
			name:        "very small totalWidth",
			screenWidth: 4, x: 4, progress: 0.5,
			wantLen: 3, wantPrefix: "[", wantSuffix: "]", wantCaret: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pb := &ProgressBar{
				screenWidth: tt.screenWidth,
				x:           tt.x,
				progress:    tt.progress,
			}
			got := pb.generateProgressLine()

			if len(got) != tt.wantLen {
				t.Errorf("len = %d, want %d; line = %q", len(got), tt.wantLen, got)
			}
			if !strings.HasPrefix(got, tt.wantPrefix) {
				t.Errorf("expected prefix %q, got %q", tt.wantPrefix, got)
			}
			if !strings.HasSuffix(got, tt.wantSuffix) {
				t.Errorf("expected suffix %q, got %q", tt.wantSuffix, got)
			}
			if tt.wantCaret {
				caretCount := strings.Count(got, "^")
				if caretCount != 1 {
					t.Errorf("expected exactly 1 caret, got %d in %q", caretCount, got)
				}
			}
			// Verify only valid characters: '[', ']', '-', '^'
			for _, r := range got {
				if r != '[' && r != ']' && r != '-' && r != '^' {
					t.Errorf("unexpected character %q in progress line %q", string(r), got)
				}
			}
		})
	}
}

func TestGenerateProgressLineCaret_Position(t *testing.T) {
	pb := &ProgressBar{
		screenWidth: 102,
		x:           2,
		progress:    0.5,
	}
	got := pb.generateProgressLine()
	// totalWidth = 100, bar content = 98 chars between '[' and ']'
	// caret at position int(98*0.5)+1 = 50 (1-indexed within bar content)
	caretIdx := strings.Index(got, "^")
	if caretIdx == -1 {
		t.Fatal("no caret found")
	}
	// caretIdx is 0-indexed in the full string; '[' is at 0
	// so caret position in bar content is caretIdx (since '[' takes index 0)
	expectedCaretIdx := 50 // 0-indexed: '[' at 0, then 49 dashes, then '^' at 50
	if caretIdx != expectedCaretIdx {
		t.Errorf("caret at index %d, want %d; line = %q", caretIdx, expectedCaretIdx, got)
	}
}

func TestGenerateBottomLine_DefaultMessage(t *testing.T) {
	pb := &ProgressBar{
		progress:    0.5,
		messageFunc: nil,
	}
	got := pb.generateBottomLine()
	expected := "Progress: 50.00%"
	if got != expected {
		t.Errorf("got %q, want %q", got, expected)
	}
}

func TestGenerateBottomLine_CustomMessage(t *testing.T) {
	called := false
	pb := &ProgressBar{
		progress: 0.75,
		messageFunc: func(progress float64, width int) string {
			called = true
			return fmt.Sprintf("Custom: %.0f%% (w=%d)", progress*100, width)
		},
		screenWidth: 80,
	}
	got := pb.generateBottomLine()
	if !called {
		t.Error("messageFunc was not called")
	}
	expected := "Custom: 75% (w=80)"
	if got != expected {
		t.Errorf("got %q, want %q", got, expected)
	}
}

func TestGenerateBottomLine_ZeroProgress(t *testing.T) {
	pb := &ProgressBar{progress: 0.0}
	got := pb.generateBottomLine()
	if got != "Progress: 0.00%" {
		t.Errorf("got %q, want %q", got, "Progress: 0.00%")
	}
}

func TestGenerateBottomLine_FullProgress(t *testing.T) {
	pb := &ProgressBar{progress: 1.0}
	got := pb.generateBottomLine()
	if got != "Progress: 100.00%" {
		t.Errorf("got %q, want %q", got, "Progress: 100.00%")
	}
}

func TestNew_ValidFrames(t *testing.T) {
	frames := [][]string{
		{"ab", "cd"},
		{"ef", "gh"},
	}
	pb, err := New(frames, nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if pb.totalFrames != 2 {
		t.Errorf("totalFrames = %d, want 2", pb.totalFrames)
	}
	if pb.x != 2 {
		t.Errorf("x = %d, want 2", pb.x)
	}
	if pb.y != 2 {
		t.Errorf("y = %d, want 2", pb.y)
	}
}

func TestNew_InconsistentHeight(t *testing.T) {
	frames := [][]string{
		{"ab", "cd"},
		{"ef"}, // only 1 row
	}
	_, err := New(frames, nil, nil)
	if err == nil {
		t.Fatal("expected error for inconsistent frame height")
	}
}

func TestNew_InconsistentWidth(t *testing.T) {
	frames := [][]string{
		{"ab", "cd"},
		{"efg", "hi"}, // first line is 3 chars
	}
	_, err := New(frames, nil, nil)
	if err == nil {
		t.Fatal("expected error for inconsistent frame width")
	}
}

func TestBasketballFrames(t *testing.T) {
	frames := Basketball()
	if len(frames) != 6 {
		t.Fatalf("expected 6 frames, got %d", len(frames))
	}
	// All frames should have same dimensions
	height := len(frames[0])
	width := len(frames[0][0])
	for i, frame := range frames {
		if len(frame) != height {
			t.Errorf("frame %d: height = %d, want %d", i, len(frame), height)
		}
		for j, line := range frame {
			if len(line) != width {
				t.Errorf("frame %d line %d: width = %d, want %d", i, j, len(line), width)
			}
		}
	}
	// Should be valid for New()
	_, err := New(frames, nil, nil)
	if err != nil {
		t.Fatalf("Basketball frames should be valid: %v", err)
	}
}
