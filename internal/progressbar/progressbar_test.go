package progressbar_test

import (
	"math/rand"
	"os"
	"testing"
	"time"

	progressbar "github.com/imafish/ddson/internal/progressbar"
)

func basketballFrames() [][]string {
	return [][]string{
		{
			`   O   `,
			`  /|\  `,
			`  / \o `,
		},
		{
			`   O   `,
			`  /|\  `,
			`  / o  `,
		},
		{
			`   O   `,
			`  /|\  `,
			`  /o\  `,
		},
		{
			`   O   `,
			`  /|\  `,
			` o/ \  `,
		},
		{
			`   O   `,
			`  /|\  `,
			`  /o\  `,
		},
		{
			`   O   `,
			`  /|\  `,
			`  / o  `,
		},
	}
}

// TestConsoleProgressBar is a visual/integration test that exercises the progress bar
// animation with real timing. Runs for ~4 seconds.
func TestConsoleProgressBar(t *testing.T) {
	pb, err := progressbar.New(basketballFrames(), os.Stdin, nil)
	if err != nil {
		t.Fatal("Failed to create progress bar:", err)
	}

	pb.Start()
	for i := 0.0; i < 1.0; i += 0.01 {
		pb.Update(i)
		time.Sleep(30 * time.Millisecond)
		time.Sleep(time.Duration(rand.Intn(20)) * time.Millisecond)
	}
	pb.Done()
}

// TestConsoleProgressBarFast is a quick smoke test that exercises the same animation
// lifecycle (New → Start → Update → Done) with minimal sleeps.
func TestConsoleProgressBarFast(t *testing.T) {
	pb, err := progressbar.New(basketballFrames(), os.Stdin, nil)
	if err != nil {
		t.Fatal("Failed to create progress bar:", err)
	}

	pb.Start()
	for i := 0.0; i < 1.0; i += 0.05 {
		pb.Update(i)
		time.Sleep(5 * time.Millisecond)
	}
	pb.Done()
}
