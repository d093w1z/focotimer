package focotimer

import (
	"sync"
	"time"
)

// ------------------- TimerData -------------------

type TimerData struct {
	mu            sync.Mutex
	Timer         *time.Timer
	Duration      time.Duration
	BreakDuration time.Duration
	IsComplete    bool
	StartedAt     time.Time
	CompletedAt   time.Time
	Handler       func()
}

func NewTimer(d time.Duration) *TimerData {
	return &TimerData{
		Duration:      d,
		BreakDuration: 1 * time.Minute,
		IsComplete:    false,
	}
}

func (t *TimerData) StartTimer() {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.Timer != nil {
		t.Timer.Stop()
	}

	t.StartedAt = time.Now()
	t.IsComplete = false

	t.Timer = time.AfterFunc(t.Duration, func() {
		t.mu.Lock()
		t.IsComplete = true
		t.CompletedAt = time.Now()
		handler := t.Handler
		t.mu.Unlock()

		if handler != nil {
			handler()
		}
	})
}

func (t *TimerData) StopTimer() {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.Timer != nil {
		t.Timer.Stop()
	}
}

func (t *TimerData) Elapsed() time.Duration {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.StartedAt.IsZero() || t.IsComplete {
		return 0
	}
	return time.Since(t.StartedAt)
}

func (t *TimerData) Remaining() time.Duration {
	elapsed := t.Elapsed()
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.Duration < elapsed {
		return 0
	}
	return t.Duration - elapsed
}
