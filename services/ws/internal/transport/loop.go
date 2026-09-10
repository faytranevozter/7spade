package transport

import (
	"errors"
	"time"

	"github.com/faytranevozter/7spade/services/ws/internal/protocol"
)

const accessCheckEvery = 5 * time.Second

type AccessChecker interface{ CheckAccess(string) error }

type Loop struct {
	PingEvery time.Duration
	PongWait  time.Duration
	Access    AccessChecker
	UserID    string
	Guest     bool
	Message   func([]byte)
	Inbound   *InboundLimiter
	Rejected  func(InboundDecision)
	Closed    func()
}

type LoopConn interface {
	ReadMessage() (int, []byte, error)
	Close() error
	StartHeartbeat(time.Duration, time.Duration) func()
}

// Run owns socket reads, heartbeats, revocation checks, and shutdown ordering.
func Run(conn LoopConn, loop Loop) {
	stopHeartbeat := conn.StartHeartbeat(loop.PingEvery, loop.PongWait)
	stopAccessCheck := StartAccessCheck(loop.Access, loop.UserID, loop.Guest, conn)
	defer func() {
		stopAccessCheck()
		stopHeartbeat()
		if loop.Closed != nil {
			loop.Closed()
		}
		_ = conn.Close()
	}()
	for {
		_, payload, err := conn.ReadMessage()
		if err != nil {
			return
		}
		if loop.Inbound != nil {
			if decision := loop.Inbound.Allow(); decision != InboundAllowed {
				if loop.Rejected != nil {
					loop.Rejected(decision)
				}
				continue
			}
		}
		if loop.Message != nil {
			loop.Message(payload)
		}
	}
}

func StartAccessCheck(checker AccessChecker, userID string, isGuest bool, conn interface{ Close() error }) func() {
	if checker == nil || isGuest {
		return func() {}
	}
	done := make(chan struct{})
	go func() {
		ticker := time.NewTicker(accessCheckEvery)
		defer ticker.Stop()
		for {
			select {
			case <-done:
				return
			case <-ticker.C:
				if errors.Is(checker.CheckAccess(userID), protocol.ErrAccessDenied) {
					_ = conn.Close()
					return
				}
			}
		}
	}()
	return func() { close(done) }
}
