package progressbar

import (
	"fmt"
	"log/slog"
	"os"
	"strings"
	"time"

	"golang.org/x/term"
)

type ProgressBar struct {
	frames      [][]string
	totalFrames int

	progress float64

	currentFrame         int
	stopChan             chan struct{}
	x                    int
	y                    int
	screenWidth          int
	screenHeight         int
	isInitialFrame       bool
	messageFunc          func(float64, int) string
	output               *os.File
	supportCursorControl bool
}

// New creates a new ProgressBar with the given frames.
func New(frames [][]string, output *os.File, messageFunc func(float64, int) string) (*ProgressBar, error) {
	x := len(frames[0][0])
	y := len(frames[0])
	for _, frame := range frames {
		if len(frame) != y {
			return nil, fmt.Errorf("all frames must have the same height")
		}
		for _, line := range frame {
			if len(line) != x {
				return nil, fmt.Errorf("all frames must have the same width")
			}
		}
	}

	return &ProgressBar{
		frames: frames,

		output:         output,
		messageFunc:    messageFunc,
		totalFrames:    len(frames),
		stopChan:       make(chan struct{}),
		x:              x,
		y:              y,
		isInitialFrame: true,
	}, nil
}

// Update updates the progress value and message.
func (pb *ProgressBar) Update(progress float64) {
	pb.progress = progress
}

// Start begins the progress bar animation.
func (pb *ProgressBar) Start() {
	fmt.Fprintln(pb.output, "")
	go func() {
		ticker := time.NewTicker(time.Duration(400) * time.Millisecond)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				pb.render()
			case <-pb.stopChan:
				return
			}
		}
	}()
}

// Done stops the progress bar.
func (pb *ProgressBar) Done() {
	close(pb.stopChan)
	fmt.Fprintln(pb.output, "")
}

func (pb *ProgressBar) render() {
	pb.getScreenProperty()
	if pb.supportCursorControl {
		pb.renderWithCursorControl()
	} else {
		pb.renderPlainText()
	}
}

// render prints the current frame and progress message.
func (pb *ProgressBar) renderWithCursorControl() {
	progressLine := pb.generateProgressLine()
	bottomLine := pb.generateBottomLine()
	offset := generateOffset(pb.screenWidth, pb.x, pb.progress)

	pb.currentFrame = (pb.currentFrame + 1) % pb.totalFrames

	if pb.isInitialFrame {
		pb.isInitialFrame = false
	} else {
		pb.moveCursor()
	}

	for _, line := range pb.frames[pb.currentFrame] {
		// print padding
		fmt.Fprint(pb.output, strings.Repeat(" ", offset))
		// print the frame line
		fmt.Fprint(pb.output, line)
		// print padding to the end of the line
		fmt.Fprintln(pb.output, strings.Repeat(" ", pb.screenWidth-offset-len(line)))
	}
	fmt.Fprint(pb.output, progressLine)
	fmt.Fprintln(pb.output, strings.Repeat(" ", pb.screenWidth-len(progressLine))) // Clear the line before printing the bottom line
	fmt.Fprint(pb.output, bottomLine)
	fmt.Fprintln(pb.output, strings.Repeat(" ", pb.screenWidth-len(bottomLine))) // Clear the line after printing the bottom line
}

func (pb *ProgressBar) renderPlainText() {
	pb.getScreenProperty()
	progressLine := pb.generateProgressLine()
	bottomLine := pb.generateBottomLine()
	fmt.Fprintln(pb.output, progressLine)
	fmt.Fprintln(pb.output, bottomLine)
}

func (pb *ProgressBar) generateProgressLine() string {
	totalWidth := pb.screenWidth - pb.x
	if totalWidth < 2 {
		totalWidth = 2 // Ensure at least two characters for the progress bar
	}

	sb := strings.Builder{}
	sb.WriteRune('[')
	currentPosition := int(float64(totalWidth-2)*pb.progress) + 1
	sb.WriteString(strings.Repeat("-", currentPosition-1))
	sb.WriteRune('^')
	sb.WriteString(strings.Repeat("-", totalWidth-currentPosition-1))
	sb.WriteRune(']')

	return sb.String()
}

func (pb *ProgressBar) moveCursor() {
	if pb.supportCursorControl {
		for i := 0; i < pb.y+2; i++ {
			fmt.Fprint(pb.output, "\033[A") // Move cursor up
		}
	}
}

func (pb *ProgressBar) generateBottomLine() string {
	if pb.messageFunc != nil {
		return pb.messageFunc(pb.progress, pb.screenWidth)
	}
	return fmt.Sprintf("Progress: %.2f%%", pb.progress*100)
}

func generateOffset(screenWidth, x int, progress float64) int {
	if progress < 0 {
		return 0
	}
	if progress > 1 {
		progress = 1
	}
	maxX := screenWidth - x - 2
	if maxX <= 0 {
		return 0
	}

	offset := int(float64(maxX)*progress) + 1
	return offset
}

func (pb *ProgressBar) getScreenProperty() {
	fd := int(pb.output.Fd())
	// Check if the file descriptor is a terminal
	if term.IsTerminal(fd) {
		pb.supportCursorControl = true
	} else {
		pb.supportCursorControl = false
	}
	// Get the size of the terminal
	width, height, err := term.GetSize(fd)
	if err != nil {
		slog.Error("Error getting terminal size", "error", err)
	}

	slog.Debug("Terminal size", "width", width, "height", height)
	pb.screenWidth = width
	pb.screenHeight = height
}
