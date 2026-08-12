package fanout

import "testing"

func TestShouldFanoutOnWrite(t *testing.T) {
	const threshold = int64(10_000)

	tests := []struct {
		name          string
		followerCount int64
		want          bool
	}{
		{"zero followers", 0, true},
		{"just below threshold", threshold - 1, true},
		{"exactly at threshold is a celebrity", threshold, false},
		{"well above threshold", threshold * 100, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ShouldFanoutOnWrite(tt.followerCount, threshold); got != tt.want {
				t.Errorf("ShouldFanoutOnWrite(%d, %d) = %v, want %v", tt.followerCount, threshold, got, tt.want)
			}
		})
	}
}
