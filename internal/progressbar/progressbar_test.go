package progressbar_test

import (
	progressbar "internal/progressbar"
	"math/rand"
	"os"
	"testing"
	"time"
)

func TestConsoleProgressBar(t *testing.T) {
	frames := [][]string{
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

	pb, err := progressbar.New(frames, os.Stdin, nil)

	if err != nil {
		t.Fatal("Failed to create progress bar:", err)
	}

	pb.Start()
	for i := 0.0; i < 1.0; i += 0.01 {
		pb.Update(i)
		time.Sleep(100 * time.Millisecond)
		time.Sleep(time.Duration(rand.Intn(100)+50) * time.Millisecond)
	}
	pb.Done()
}
