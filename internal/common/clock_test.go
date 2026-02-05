package common

import "testing"

func TestRealClockNow(t *testing.T) {
	c := RealClock{}
	now := c.Now()
	if now.IsZero() {
		t.Fatalf("expected non-zero time")
	}
}
