package focotimer

import (
	"sync"
	"time"
)

type TimerManager struct {
	mu        sync.Mutex
	subs      []chan time.Duration
	Timer     *TimerData
	lastValue time.Duration
	updates   chan time.Duration
	stopCh    chan struct{}
	doneCh    chan struct{}
}

var GTimerManager = NewTimerManager(10 * time.Second)

func NewTimerManager(duration time.Duration) *TimerManager {
	tm := &TimerManager{
		Timer:   NewTimer(duration),
		updates: make(chan time.Duration),
		stopCh:  make(chan struct{}),
		doneCh:  make(chan struct{}),
	}
	go tm.broadcast() // single broadcaster goroutine
	return tm
}

// --- Subscriptions ---

func (t *TimerManager) Subscribe() <-chan time.Duration {
	ch := make(chan time.Duration, 10)
	t.mu.Lock()
	t.subs = append(t.subs, ch)
	t.mu.Unlock()
	return ch
}

func (t *TimerManager) broadcast() {
	ticker := time.NewTicker(200 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-t.stopCh:
			return
		case <-ticker.C:
			remaining := t.Timer.Remaining()
			t.mu.Lock()
			t.lastValue = remaining
			for _, ch := range t.subs {
				select {
				case ch <- remaining:
				default: // drop if slow
				}
			}
			t.mu.Unlock()
		}
	}
}

// --- Control methods ---

func (t *TimerManager) Stop() {
	t.Timer.StopTimer()
}

func (t *TimerManager) Reset() {
	t.mu.Lock()
	defer t.mu.Unlock()

	d := t.Timer.Duration
	t.Timer = NewTimer(d)
	t.lastValue = d

	// replace with a fresh done channel
	t.doneCh = make(chan struct{})
}

func (t *TimerManager) Start() {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.Timer != nil {
		// hook completion into TimerData
		t.Timer.Handler = func() {
			t.mu.Lock()
			defer t.mu.Unlock()
			select {
			case <-t.doneCh:
				// already closed
			default:
				close(t.doneCh) // fire done
			}
		}
		t.Timer.StartTimer()
	}
}

func (t *TimerManager) Inc() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.Timer.Duration += 5 * time.Second
}

func (t *TimerManager) Dec() {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.Timer.Duration > 5*time.Second {
		t.Timer.Duration -= 5 * time.Second
	} else {
		t.Timer.Duration = 0
	}
}

func (t *TimerManager) Snapshot() time.Duration {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.lastValue
}

func (t *TimerManager) Done() <-chan struct{} {
	return t.doneCh
}
