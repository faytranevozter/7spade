package transport

import (
	"sync"
	"time"
)

const (
	inboundFloodWindow = 10 * time.Second
	inboundFloodLimit  = 40
	inboundFloodClose  = 80
)

type InboundDecision uint8

const (
	InboundAllowed InboundDecision = iota
	InboundSlowDown
	InboundClose
)

// InboundLimiter protects one client connection from inbound frame floods.
type InboundLimiter struct {
	mu sync.Mutex
	at []time.Time
}

func (l *InboundLimiter) Allow() InboundDecision {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := time.Now()
	cutoff := now.Add(-inboundFloodWindow)
	kept := l.at[:0]
	for _, at := range l.at {
		if !at.Before(cutoff) {
			kept = append(kept, at)
		}
	}
	l.at = append(kept, now)

	switch n := len(l.at); {
	case n > inboundFloodClose:
		return InboundClose
	case n > inboundFloodLimit:
		return InboundSlowDown
	default:
		return InboundAllowed
	}
}
