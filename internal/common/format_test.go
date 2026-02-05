package common

import "testing"

func TestPrettyFormatSize(t *testing.T) {
	cases := []struct {
		in   int64
		want string
	}{
		{0, "0 B"},
		{512, "512 B"},
		{1024, "1.00 KB"},
		{1024 * 1024, "1.00 MB"},
		{1024 * 1024 * 1024, "1.00 GB"},
	}

	for _, tc := range cases {
		if got := PrettyFormatSize(tc.in); got != tc.want {
			t.Fatalf("PrettyFormatSize(%d) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestPrettyFormatSpeed(t *testing.T) {
	cases := []struct {
		in   int
		want string
	}{
		{0, "0 B/s"},
		{512, "512 B/s"},
		{1024, "1.00 KB/s"},
		{1024 * 1024, "1.00 MB/s"},
		{1024 * 1024 * 1024, "1.00 GB/s"},
	}

	for _, tc := range cases {
		if got := PrettyFormatSpeed(tc.in); got != tc.want {
			t.Fatalf("PrettyFormatSpeed(%d) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestPrettyFormatDuration(t *testing.T) {
	if got := PrettyFormatDuration(1000, 0); got != "N/A" {
		t.Fatalf("expected N/A, got %q", got)
	}

	// duration=120, speed=2 => 60 seconds
	if got := PrettyFormatDuration(120, 2); got != "00:01:00" {
		t.Fatalf("expected 00:01:00, got %q", got)
	}
}
