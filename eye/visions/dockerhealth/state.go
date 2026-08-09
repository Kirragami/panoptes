package dockerhealth

import (
	"sync"
	"time"
)

type dockerState struct {
	mu sync.Mutex

	omenRaised    bool
	lastAwakening time.Time
}

func newDockerState() *dockerState {
	return &dockerState{}
}

func (s *dockerState) awakenNow() {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.omenRaised = false
	s.lastAwakening = time.Now()
}

func (s *dockerState) clearOmen() {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.omenRaised = false
}

func (s *dockerState) withinGrace(
	grace time.Duration,
) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.lastAwakening.IsZero() {
		return false
	}

	return time.Since(s.lastAwakening) < grace
}

func (s *dockerState) claimOmen() bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.omenRaised {
		return false
	}

	s.omenRaised = true
	return true
}

func (s *dockerState) releaseOmen() {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.omenRaised = false
}