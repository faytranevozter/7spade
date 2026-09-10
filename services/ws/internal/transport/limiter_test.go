package transport

import (
	"testing"
	"time"
)

func TestInboundLimiterFloodThresholds(t *testing.T) {
	limiter := &InboundLimiter{}
	for range inboundFloodLimit {
		if got := limiter.Allow(); got != InboundAllowed {
			t.Fatalf("under limit = %v, want allowed", got)
		}
	}
	if got := limiter.Allow(); got != InboundSlowDown {
		t.Fatalf("over limit = %v, want slow down", got)
	}
	for range inboundFloodClose - inboundFloodLimit - 1 {
		limiter.Allow()
	}
	if got := limiter.Allow(); got != InboundClose {
		t.Fatalf("close threshold = %v, want close", got)
	}
}

func TestInboundLimiterWindowResets(t *testing.T) {
	limiter := &InboundLimiter{}
	old := time.Now().Add(-inboundFloodWindow - time.Second)
	limiter.at = make([]time.Time, inboundFloodLimit+5)
	for i := range limiter.at {
		limiter.at[i] = old
	}
	if got := limiter.Allow(); got != InboundAllowed {
		t.Fatalf("after window expiry = %v, want allowed", got)
	}
	if got := len(limiter.at); got != 1 {
		t.Fatalf("recorded timestamps = %d, want 1 after prune", got)
	}
}
