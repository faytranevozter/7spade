package repository

import (
	"testing"
	"time"
)

func TestEventStatusBoundaries(t *testing.T) {
	start := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	end := start.Add(24 * time.Hour)
	tests := []struct {
		name string
		now  time.Time
		want string
	}{
		{"before", start.Add(-time.Nanosecond), "upcoming"},
		{"at start", start, "active"},
		{"before end", end.Add(-time.Nanosecond), "active"},
		{"at end", end, "ended"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := eventStatus(tt.now, start, end); got != tt.want {
				t.Fatalf("eventStatus() = %q, want %q", got, tt.want)
			}
		})
	}
}
