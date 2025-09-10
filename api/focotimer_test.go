package focotimer

import (
	"sync"
	"testing"
	"time"
)

// ================= TimerData Tests =================

func TestNewTimer(t *testing.T) {
	duration := 5 * time.Second
	timer := NewTimer(duration)

	if timer.Duration != duration {
		t.Errorf("Expected duration %v, got %v", duration, timer.Duration)
	}
	if timer.BreakDuration != 1*time.Minute {
		t.Errorf("Expected break duration %v, got %v", 1*time.Minute, timer.BreakDuration)
	}
	if timer.IsComplete != false {
		t.Errorf("Expected IsComplete to be false, got %v", timer.IsComplete)
	}
	if timer.Timer != nil {
		t.Errorf("Expected Timer to be nil initially, got %v", timer.Timer)
	}
}

func TestTimerData_StartTimer(t *testing.T) {
	timer := NewTimer(100 * time.Millisecond)

	// Test that timer starts
	timer.StartTimer()
	if timer.Timer == nil {
		t.Fatal("Expected Timer to be set after StartTimer")
	}
	if timer.StartedAt.IsZero() {
		t.Fatal("Expected StartedAt to be set after StartTimer")
	}
	if timer.IsComplete {
		t.Fatal("Expected IsComplete to be false after StartTimer")
	}

	// Wait for timer to complete
	time.Sleep(150 * time.Millisecond)

	if !timer.IsComplete {
		t.Error("Expected IsComplete to be true after timer completion")
	}
	if timer.CompletedAt.IsZero() {
		t.Error("Expected CompletedAt to be set after timer completion")
	}
}

func TestTimerData_StartTimer_WithHandler(t *testing.T) {
	timer := NewTimer(50 * time.Millisecond)

	var handlerCalled bool
	var mu sync.Mutex

	timer.Handler = func() {
		mu.Lock()
		handlerCalled = true
		mu.Unlock()
	}

	timer.StartTimer()
	time.Sleep(100 * time.Millisecond)

	mu.Lock()
	called := handlerCalled
	mu.Unlock()

	if !called {
		t.Error("Expected handler to be called after timer completion")
	}
}

func TestTimerData_StopTimer(t *testing.T) {
	timer := NewTimer(1 * time.Second)
	timer.StartTimer()

	// Stop the timer before completion
	time.Sleep(10 * time.Millisecond)
	timer.StopTimer()

	// Wait a bit more to ensure it doesn't complete
	time.Sleep(100 * time.Millisecond)

	if timer.IsComplete {
		t.Error("Expected timer to not complete after StopTimer")
	}
}

func TestTimerData_Elapsed(t *testing.T) {
	timer := NewTimer(1 * time.Second)

	// Test before starting
	elapsed := timer.Elapsed()
	if elapsed != 0 {
		t.Errorf("Expected elapsed to be 0 before starting, got %v", elapsed)
	}

	// Test after starting
	timer.StartTimer()
	time.Sleep(100 * time.Millisecond)
	elapsed = timer.Elapsed()

	if elapsed < 90*time.Millisecond || elapsed > 150*time.Millisecond {
		t.Errorf("Expected elapsed to be around 100ms, got %v", elapsed)
	}

	// Test after completion
	timer = NewTimer(10 * time.Millisecond)
	timer.StartTimer()
	time.Sleep(50 * time.Millisecond)
	elapsed = timer.Elapsed()

	if elapsed != 0 {
		t.Errorf("Expected elapsed to be 0 after completion, got %v", elapsed)
	}
}

func TestTimerData_Remaining(t *testing.T) {
	duration := 200 * time.Millisecond
	timer := NewTimer(duration)

	// Test before starting
	remaining := timer.Remaining()
	if remaining != 0 {
		t.Errorf("Expected remaining to be 0 before starting, got %v", remaining)
	}

	// Test after starting
	timer.StartTimer()
	time.Sleep(50 * time.Millisecond)
	remaining = timer.Remaining()

	expected := duration - 50*time.Millisecond
	tolerance := 50 * time.Millisecond

	if remaining < expected-tolerance || remaining > expected+tolerance {
		t.Errorf("Expected remaining to be around %v, got %v", expected, remaining)
	}

	// Test after completion
	time.Sleep(200 * time.Millisecond)
	remaining = timer.Remaining()

	if remaining != 0 {
		t.Errorf("Expected remaining to be 0 after completion, got %v", remaining)
	}
}

func TestTimerData_ConcurrentAccess(t *testing.T) {
	timer := NewTimer(100 * time.Millisecond)

	var wg sync.WaitGroup

	// Start multiple goroutines accessing the timer
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			timer.StartTimer()
			timer.Elapsed()
			timer.Remaining()
			timer.StopTimer()
		}()
	}

	wg.Wait() // Should not panic or deadlock
}

// ================= TimerManager Tests =================

func TestNewTimerManager(t *testing.T) {
	duration := 5 * time.Second
	tm := NewTimerManager(duration)

	if tm.Timer == nil {
		t.Fatal("Expected Timer to be initialized")
	}
	if tm.Timer.Duration != duration {
		t.Errorf("Expected timer duration %v, got %v", duration, tm.Timer.Duration)
	}
	if tm.updates == nil {
		t.Fatal("Expected updates channel to be initialized")
	}
	if tm.stopCh == nil {
		t.Fatal("Expected stopCh to be initialized")
	}
	if tm.doneCh == nil {
		t.Fatal("Expected doneCh to be initialized")
	}
}

func TestTimerManager_Subscribe(t *testing.T) {
	tm := NewTimerManager(1 * time.Second)
	defer func() {
		close(tm.stopCh)
	}()

	ch := tm.Subscribe()
	if ch == nil {
		t.Fatal("Expected subscription channel to be returned")
	}

	tm.mu.Lock()
	subCount := len(tm.subs)
	tm.mu.Unlock()

	if subCount != 1 {
		t.Errorf("Expected 1 subscriber, got %d", subCount)
	}

	// Test multiple subscriptions
	ch2 := tm.Subscribe()
	if ch2 == nil {
		t.Fatal("Expected second subscription channel to be returned")
	}

	tm.mu.Lock()
	subCount = len(tm.subs)
	tm.mu.Unlock()

	if subCount != 2 {
		t.Errorf("Expected 2 subscribers, got %d", subCount)
	}
}

func TestTimerManager_Broadcast(t *testing.T) {
	tm := NewTimerManager(500 * time.Millisecond)
	defer func() {
		close(tm.stopCh)
	}()

	ch := tm.Subscribe()

	// Start the timer to get meaningful values
	tm.Start()

	// Should receive updates
	select {
	case remaining := <-ch:
		if remaining <= 0 || remaining > 500*time.Millisecond {
			t.Errorf("Expected remaining time between 0 and 500ms, got %v", remaining)
		}
	case <-time.After(1 * time.Second):
		t.Error("Expected to receive broadcast update within 1 second")
	}
}

func TestTimerManager_Start(t *testing.T) {
	tm := NewTimerManager(100 * time.Millisecond)
	defer func() {
		close(tm.stopCh)
	}()

	tm.Start()

	if tm.Timer.Timer == nil {
		t.Error("Expected internal timer to be started")
	}
	if tm.Timer.StartedAt.IsZero() {
		t.Error("Expected StartedAt to be set")
	}
}

func TestTimerManager_Stop(t *testing.T) {
	tm := NewTimerManager(1 * time.Second)
	defer func() {
		close(tm.stopCh)
	}()

	tm.Start()
	time.Sleep(10 * time.Millisecond)
	tm.Stop()

	// Timer should be stopped, so it shouldn't complete
	time.Sleep(100 * time.Millisecond)
	if tm.Timer.IsComplete {
		t.Error("Expected timer to not complete after Stop")
	}
}

func TestTimerManager_Reset(t *testing.T) {
	tm := NewTimerManager(100 * time.Millisecond)
	defer func() {
		close(tm.stopCh)
	}()

	originalDuration := tm.Timer.Duration
	oldDoneCh := tm.doneCh

	tm.Start()
	time.Sleep(10 * time.Millisecond)

	tm.Reset()

	if tm.Timer.Duration != originalDuration {
		t.Errorf("Expected duration to be preserved after reset, got %v", tm.Timer.Duration)
	}
	if tm.Timer.StartedAt != (time.Time{}) {
		t.Error("Expected StartedAt to be reset")
	}
	if tm.Timer.IsComplete {
		t.Error("Expected IsComplete to be false after reset")
	}
	if tm.doneCh == oldDoneCh {
		t.Error("Expected doneCh to be replaced after reset")
	}
}

func TestTimerManager_Inc(t *testing.T) {
	tm := NewTimerManager(100 * time.Millisecond)
	defer func() {
		close(tm.stopCh)
	}()

	originalDuration := tm.Timer.Duration
	tm.Inc()

	expectedDuration := originalDuration + 5*time.Second
	if tm.Timer.Duration != expectedDuration {
		t.Errorf("Expected duration %v after Inc, got %v", expectedDuration, tm.Timer.Duration)
	}
}

func TestTimerManager_Dec(t *testing.T) {
	tm := NewTimerManager(10 * time.Second)
	defer func() {
		close(tm.stopCh)
	}()

	originalDuration := tm.Timer.Duration
	tm.Dec()

	expectedDuration := originalDuration - 5*time.Second
	if tm.Timer.Duration != expectedDuration {
		t.Errorf("Expected duration %v after Dec, got %v", expectedDuration, tm.Timer.Duration)
	}
}

func TestTimerManager_Dec_MinimumZero(t *testing.T) {
	tm := NewTimerManager(3 * time.Second)
	defer func() {
		close(tm.stopCh)
	}()

	tm.Dec() // Should not go below 0

	if tm.Timer.Duration != 0 {
		t.Errorf("Expected duration to be 0 when decreasing below 5 seconds, got %v", tm.Timer.Duration)
	}
}

func TestTimerManager_Snapshot(t *testing.T) {
	tm := NewTimerManager(200 * time.Millisecond)
	defer func() {
		close(tm.stopCh)
	}()

	tm.Start()
	time.Sleep(50 * time.Millisecond)

	// Give the broadcast goroutine time to update
	time.Sleep(250 * time.Millisecond)

	snapshot := tm.Snapshot()

	// Should be less than original duration but greater than 0 (unless completed)
	if snapshot < 0 || snapshot > 200*time.Millisecond {
		t.Errorf("Expected snapshot to be between 0 and 200ms, got %v", snapshot)
	}
}

func TestTimerManager_Done(t *testing.T) {
	tm := NewTimerManager(50 * time.Millisecond)
	defer func() {
		close(tm.stopCh)
	}()

	doneCh := tm.Done()
	tm.Start()

	// Should receive on done channel when timer completes
	select {
	case <-doneCh:
		// Expected
	case <-time.After(200 * time.Millisecond):
		t.Error("Expected done channel to be closed when timer completes")
	}
}

func TestTimerManager_Done_Reset(t *testing.T) {
	tm := NewTimerManager(50 * time.Millisecond)
	defer func() {
		close(tm.stopCh)
	}()

	doneCh1 := tm.Done()
	tm.Reset()
	doneCh2 := tm.Done()

	if doneCh1 == doneCh2 {
		t.Error("Expected done channel to be different after reset")
	}
}

func TestTimerManager_ConcurrentAccess(t *testing.T) {
	tm := NewTimerManager(100 * time.Millisecond)
	defer func() {
		close(tm.stopCh)
	}()

	var wg sync.WaitGroup

	// Test concurrent access to all methods
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			tm.Start()
			tm.Stop()
			tm.Inc()
			tm.Dec()
			tm.Snapshot()
			tm.Reset()
			tm.Subscribe()
			tm.Done()
		}()
	}

	wg.Wait() // Should not panic or deadlock
}

func TestTimerManager_MultipleSubscribers(t *testing.T) {
	tm := NewTimerManager(200 * time.Millisecond)
	defer func() {
		close(tm.stopCh)
	}()

	// Create multiple subscribers
	ch1 := tm.Subscribe()
	ch2 := tm.Subscribe()
	ch3 := tm.Subscribe()

	tm.Start()

	// All subscribers should receive updates
	timeout := time.After(1 * time.Second)

	for i, ch := range []<-chan time.Duration{ch1, ch2, ch3} {
		select {
		case remaining := <-ch:
			if remaining < 0 || remaining > 200*time.Millisecond {
				t.Errorf("Subscriber %d received invalid remaining time: %v", i, remaining)
			}
		case <-timeout:
			t.Errorf("Subscriber %d did not receive update within timeout", i)
		}
	}
}

func TestGlobalTimerManager(t *testing.T) {
	if GTimerManager == nil {
		t.Fatal("Expected GTimerManager to be initialized")
	}

	expectedDuration := 10 * time.Second
	if GTimerManager.Timer.Duration != expectedDuration {
		t.Errorf("Expected GTimerManager duration to be %v, got %v",
			expectedDuration, GTimerManager.Timer.Duration)
	}
}

// ================= Integration Tests =================

func TestTimerManager_FullWorkflow(t *testing.T) {
	tm := NewTimerManager(100 * time.Millisecond)
	defer func() {
		close(tm.stopCh)
	}()

	// Subscribe to updates
	ch := tm.Subscribe()
	doneCh := tm.Done()

	// Start timer
	tm.Start()

	// Should receive at least one update
	select {
	case remaining := <-ch:
		if remaining <= 0 || remaining > 100*time.Millisecond {
			t.Errorf("Expected valid remaining time, got %v", remaining)
		}
	case <-time.After(500 * time.Millisecond):
		t.Error("Expected to receive at least one update")
	}

	// Should complete
	select {
	case <-doneCh:
		// Expected
	case <-time.After(500 * time.Millisecond):
		t.Error("Expected timer to complete")
	}

	// Snapshot should show completion
	time.Sleep(10 * time.Millisecond) // Give broadcast time to update
	snapshot := tm.Snapshot()
	if snapshot != 0 {
		t.Errorf("Expected snapshot to be 0 after completion, got %v", snapshot)
	}
}

func TestTimerManager_IncDecWorkflow(t *testing.T) {
	tm := NewTimerManager(100 * time.Millisecond)
	defer func() {
		close(tm.stopCh)
	}()

	originalDuration := tm.Timer.Duration

	// Increase duration
	tm.Inc()
	tm.Inc()
	expectedDuration := originalDuration + 10*time.Second
	if tm.Timer.Duration != expectedDuration {
		t.Errorf("Expected duration %v after 2 Inc, got %v", expectedDuration, tm.Timer.Duration)
	}

	// Decrease duration
	tm.Dec()
	expectedDuration = originalDuration + 5*time.Second
	if tm.Timer.Duration != expectedDuration {
		t.Errorf("Expected duration %v after 1 Dec, got %v", expectedDuration, tm.Timer.Duration)
	}

	// Reset should restore original duration
	tm.Reset()
	if tm.Timer.Duration != originalDuration {
		t.Errorf("Expected duration to be restored to %v after Reset, got %v",
			originalDuration, tm.Timer.Duration)
	}
}
