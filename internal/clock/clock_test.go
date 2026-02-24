package clock

import "testing"

func TestRealClockNow(t *testing.T) {
	clk := RealClock{}
	if clk.Now().IsZero() {
		t.Fatal("expected non-zero current time")
	}
}
