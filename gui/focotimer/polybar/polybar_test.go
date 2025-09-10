package polybar

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	focotimer "github.com/d093w1z/focotimer/api"
)

// Test helpers
func setupTempDir(t *testing.T) string {
	tmpDir := t.TempDir()
	return tmpDir
}

func waitForFile(path string, timeout time.Duration) bool {
	start := time.Now()
	for time.Since(start) < timeout {
		if _, err := os.Stat(path); err == nil {
			return true
		}
		time.Sleep(10 * time.Millisecond)
	}
	return false
}

func writeToFifo(t *testing.T, path, data string) {
	file, err := os.OpenFile(path, os.O_WRONLY, 0)
	if err != nil {
		t.Fatalf("Failed to open FIFO for writing: %v", err)
	}
	defer file.Close()

	if _, err := io.WriteString(file, data); err != nil {
		t.Fatalf("Failed to write to FIFO: %v", err)
	}
}

// ================= Setup/Teardown Tests =================

func TestInit(t *testing.T) {
	// Reset global state
	fifoPipePath = ""

	tmpDir := setupTempDir(t)
	basePipe := filepath.Join(tmpDir, "test.pipe")

	// Set environment variable
	oldEnv := os.Getenv("FOCOTIMER_PIPE")
	os.Setenv("FOCOTIMER_PIPE", basePipe)
	defer os.Setenv("FOCOTIMER_PIPE", oldEnv)

	Init()

	if fifoPipePath == "" {
		t.Fatal("Expected fifoPipePath to be set after Init")
	}

	// Should contain PID to make it unique
	pid := os.Getpid()
	expectedPattern := fmt.Sprintf("%s.%d", basePipe, pid)
	if !strings.HasPrefix(fifoPipePath, expectedPattern) {
		t.Errorf("Expected FIFO path to start with %s, got %s", expectedPattern, fifoPipePath)
	}

	// File should exist and be a named pipe
	if !waitForFile(fifoPipePath, 1*time.Second) {
		t.Fatal("FIFO file was not created")
	}

	fi, err := os.Stat(fifoPipePath)
	if err != nil {
		t.Fatalf("Failed to stat FIFO: %v", err)
	}

	if fi.Mode()&os.ModeNamedPipe == 0 {
		t.Error("Created file is not a named pipe")
	}
}

func TestInitWithBase(t *testing.T) {
	tmpDir := setupTempDir(t)
	basePipe := filepath.Join(tmpDir, "custom.pipe")

	path, err := InitWithBase(basePipe)
	if err != nil {
		t.Fatalf("InitWithBase failed: %v", err)
	}

	if path == "" {
		t.Fatal("Expected non-empty path from InitWithBase")
	}

	// Should contain PID
	pid := os.Getpid()
	expectedPattern := fmt.Sprintf("%s.%d", basePipe, pid)
	if !strings.HasPrefix(path, expectedPattern) {
		t.Errorf("Expected path to start with %s, got %s", expectedPattern, path)
	}

	// Clean up
	os.Remove(path)
}

func TestMkfifoUnique(t *testing.T) {
	tmpDir := setupTempDir(t)
	basePath := filepath.Join(tmpDir, "unique.pipe")

	// First call should succeed
	path1, err := mkfifoUnique(basePath, 0666)
	if err != nil {
		t.Fatalf("First mkfifoUnique call failed: %v", err)
	}
	defer os.Remove(path1)

	// Should contain PID
	pid := os.Getpid()
	expectedPattern := fmt.Sprintf("%s.%d", basePath, pid)
	if !strings.HasPrefix(path1, expectedPattern) {
		t.Errorf("Expected path to start with %s, got %s", expectedPattern, path1)
	}

	// Second call should return different path or reuse if available
	path2, err := mkfifoUnique(basePath, 0666)
	if err != nil {
		t.Fatalf("Second mkfifoUnique call failed: %v", err)
	}
	defer os.Remove(path2)

	// Both should be valid named pipes
	for i, path := range []string{path1, path2} {
		fi, err := os.Stat(path)
		if err != nil {
			t.Fatalf("Failed to stat path %d (%s): %v", i, path, err)
		}
		if fi.Mode()&os.ModeNamedPipe == 0 {
			t.Errorf("Path %d (%s) is not a named pipe", i, path)
		}
	}
}

func TestCanUseFifo(t *testing.T) {
	tmpDir := setupTempDir(t)
	fifoPath := filepath.Join(tmpDir, "test.pipe")

	// Create a FIFO
	path, err := mkfifoUnique(fifoPath, 0666)
	if err != nil {
		t.Fatalf("Failed to create FIFO: %v", err)
	}
	defer os.Remove(path)

	// Should be usable initially
	if !canUseFifo(path) {
		t.Error("Expected FIFO to be usable when not in use")
	}
}

// ================= Handler Tests =================

func TestAddHandler(t *testing.T) {
	var called bool
	var mu sync.Mutex

	handler := func() {
		mu.Lock()
		called = true
		mu.Unlock()
	}

	AddHandler(handler)

	// Verify handler was stored
	// mu.RLock()
	storedHandler := guiToggleCallback
	// mu2.RUnlock()

	if storedHandler == nil {
		t.Fatal("Expected handler to be stored")
	}

	// Call the stored handler
	storedHandler()

	mu.Lock()
	wasCalled := called
	mu.Unlock()

	if !wasCalled {
		t.Error("Expected handler to be called")
	}
}

// ================= TimerManager Integration Tests =================

func TestSetTimerManager(t *testing.T) {
	tm := focotimer.NewTimerManager(5 * time.Second)

	SetTimerManager(tm)

	retrieved := getTimerManager()
	if retrieved != tm {
		t.Error("Expected retrieved TimerManager to match the one set")
	}
}

func TestGetTimerManager_Nil(t *testing.T) {
	// Reset global state
	SetTimerManager(nil)

	retrieved := getTimerManager()
	if retrieved != nil {
		t.Error("Expected getTimerManager to return nil when none set")
	}
}

func TestTimerWrappers_WithManager(t *testing.T) {
	tm := focotimer.NewTimerManager(100 * time.Millisecond)
	SetTimerManager(tm)

	// Test all wrapper functions
	TimerStart()
	if tm.Timer.Timer == nil {
		t.Error("Expected timer to be started after TimerStart")
	}

	TimerInc()
	if tm.Timer.Duration != 100*time.Millisecond+5*time.Second {
		t.Error("Expected timer duration to be increased after TimerInc")
	}

	TimerDec()
	if tm.Timer.Duration != 100*time.Millisecond {
		t.Error("Expected timer duration to be decreased after TimerDec")
	}

	snapshot := Snapshot()
	if snapshot < 0 || snapshot > 100*time.Millisecond {
		t.Errorf("Expected valid snapshot, got %v", snapshot)
	}

	ch := Subscribe()
	if ch == nil {
		t.Error("Expected Subscribe to return a channel")
	}

	TimerStop()
	// Should not panic or error
}

func TestTimerWrappers_WithoutManager(t *testing.T) {
	// Reset global state
	SetTimerManager(nil)

	// All functions should handle nil manager gracefully
	TimerStart() // Should not panic
	TimerStop()  // Should not panic
	TimerInc()   // Should not panic
	TimerDec()   // Should not panic

	snapshot := Snapshot()
	if snapshot != 0 {
		t.Errorf("Expected Snapshot to return 0 with nil manager, got %v", snapshot)
	}

	ch := Subscribe()
	if ch != nil {
		t.Error("Expected Subscribe to return nil with nil manager")
	}
}

// ================= Output Tests =================

func TestPolybarActionButton(t *testing.T) {
	button := "Test Button"
	action := "test_action"

	result := polybarActionButton(button, action)
	expected := "%{A:test_action:} Test Button %{A}"

	if result != expected {
		t.Errorf("Expected %q, got %q", expected, result)
	}
}

func TestPolybarActionButton_WithNewline(t *testing.T) {
	button := "Test Button\n"
	action := "test_action"

	result := polybarActionButton(button, action)
	expected := "%{A:test_action:} Test Button %{A}"

	if result != expected {
		t.Errorf("Expected newline to be stripped: %q, got %q", expected, result)
	}
}

func TestPipeCommand(t *testing.T) {
	fifoPipePath = "/tmp/test.pipe"
	cmd := "start"

	result := pipeCommand(cmd)
	expected := "echo 'start' > /tmp/test.pipe"

	if result != expected {
		t.Errorf("Expected %q, got %q", expected, result)
	}
}

func TestOutput(t *testing.T) {
	// Set up a timer manager with known values
	tm := focotimer.NewTimerManager(300 * time.Second)
	SetTimerManager(tm)
	fifoPipePath = "/tmp/test.pipe"

	result := output()

	// Should contain the expected button structure
	if !strings.Contains(result, "[-]") {
		t.Error("Expected output to contain dec button")
	}
	if !strings.Contains(result, "[+]") {
		t.Error("Expected output to contain inc button")
	}
	if !strings.Contains(result, "5m0s : 5m0s") {
		t.Error("Expected output to contain time display")
	}
	if !strings.Contains(result, "%{A:") {
		t.Error("Expected output to contain polybar action syntax")
	}
}

func TestTruncToSecond(t *testing.T) {
	tests := []struct {
		input    time.Duration
		expected time.Duration
	}{
		{1500 * time.Millisecond, 1 * time.Second},
		{2750 * time.Millisecond, 2 * time.Second},
		{-1500 * time.Millisecond, -1 * time.Second},
		{-2750 * time.Millisecond, -2 * time.Second},
		{0, 0},
		{500 * time.Millisecond, 0},
	}

	for _, test := range tests {
		result := truncToSecond(test.input)
		if result != test.expected {
			t.Errorf("truncToSecond(%v) = %v, expected %v", test.input, result, test.expected)
		}
	}
}

func TestTimerSnapshot(t *testing.T) {
	// Test with nil manager
	SetTimerManager(nil)
	dur, rem := timerSnapshot()
	if dur != 0 || rem != 0 {
		t.Errorf("Expected (0, 0) with nil manager, got (%v, %v)", dur, rem)
	}

	// Test with actual manager
	tm := focotimer.NewTimerManager(200 * time.Second)
	SetTimerManager(tm)
	tm.Start()

	dur, rem = timerSnapshot()
	if dur != 200*time.Second {
		t.Errorf("Expected duration 200s, got %v", dur)
	}
	if rem < 0 || rem > 200*time.Second {
		t.Errorf("Expected remaining between 0 and 200s, got %v", rem)
	}
}

// ================= Command Handling Tests =================

func TestFifoPath(t *testing.T) {
	expectedPath := "/tmp/test.pipe"
	fifoPipePath = expectedPath

	result := FifoPath()
	if result != expectedPath {
		t.Errorf("Expected %q, got %q", expectedPath, result)
	}
}

func TestHandleCmds_Commands(t *testing.T) {
	tmpDir := setupTempDir(t)
	basePipe := filepath.Join(tmpDir, "test.pipe")

	path, err := InitWithBase(basePipe)
	if err != nil {
		t.Fatalf("Failed to initialize FIFO: %v", err)
	}
	defer os.Remove(path)

	// Set up timer manager
	tm := focotimer.NewTimerManager(100 * time.Millisecond)
	SetTimerManager(tm)

	// Set up handler
	var guiCalled bool
	var guiMu sync.Mutex
	AddHandler(func() {
		guiMu.Lock()
		guiCalled = true
		guiMu.Unlock()
	})

	// Start command handler in background
	go func() {
		wg.Add(1)
		defer wg.Done()
		handle_cmds()
	}()

	// Give handler time to start
	time.Sleep(50 * time.Millisecond)

	tests := []struct {
		command        string
		expectedEffect func() bool
		description    string
	}{
		{
			command: "start",
			expectedEffect: func() bool {
				return tm.Timer.Timer != nil && !tm.Timer.StartedAt.IsZero()
			},
			description: "timer should be started",
		},
		{
			command: "gui",
			expectedEffect: func() bool {
				guiMu.Lock()
				called := guiCalled
				guiMu.Unlock()
				return called
			},
			description: "GUI callback should be called",
		},
		{
			command: "inc",
			expectedEffect: func() bool {
				return tm.Timer.Duration > 100*time.Millisecond
			},
			description: "timer duration should be increased",
		},
		{
			command: "stop",
			expectedEffect: func() bool {
				// Reset timer first to have a clean state
				tm.Reset()
				tm.Start()
				time.Sleep(10 * time.Millisecond)

				// Send stop command
				go writeToFifo(t, path, "stop")
				time.Sleep(50 * time.Millisecond)

				// Timer should not be complete after stop
				return !tm.Timer.IsComplete
			},
			description: "timer should be stopped",
		},
	}

	for _, test := range tests {
		t.Run(test.command, func(t *testing.T) {
			if test.command == "stop" {
				// Special handling for stop command test
				if !test.expectedEffect() {
					t.Errorf("Command %s failed: %s", test.command, test.description)
				}
				return
			}

			// Send command
			go writeToFifo(t, path, test.command)

			// Wait for command to be processed
			time.Sleep(100 * time.Millisecond)

			if !test.expectedEffect() {
				t.Errorf("Command %s failed: %s", test.command, test.description)
			}
		})
	}

	// Signal shutdown
	close(stopping)
	wg.Wait()
}

func TestHandleCmds_UnknownCommand(t *testing.T) {
	tmpDir := setupTempDir(t)
	basePipe := filepath.Join(tmpDir, "test.pipe")

	path, err := InitWithBase(basePipe)
	if err != nil {
		t.Fatalf("Failed to initialize FIFO: %v", err)
	}
	defer os.Remove(path)

	// Start command handler in background
	go func() {
		wg.Add(1)
		defer wg.Done()
		handle_cmds()
	}()

	// Give handler time to start
	time.Sleep(50 * time.Millisecond)

	// Send unknown command - should not panic
	go writeToFifo(t, path, "unknown_command")
	time.Sleep(100 * time.Millisecond)

	// Signal shutdown
	close(stopping)
	wg.Wait()
}

// ================= Shutdown Tests =================

func TestShutdown(t *testing.T) {
	tmpDir := setupTempDir(t)
	basePipe := filepath.Join(tmpDir, "shutdown_test.pipe")

	path, err := InitWithBase(basePipe)
	if err != nil {
		t.Fatalf("Failed to initialize FIFO: %v", err)
	}

	// Verify file exists before shutdown
	if !waitForFile(path, 1*time.Second) {
		t.Fatal("FIFO file should exist before shutdown")
	}

	// Reset the sync.Once variables to test shutdown
	stopOnce = sync.Once{}
	stopping = make(chan struct{})

	Shutdown()

	// Verify file is removed after shutdown
	time.Sleep(100 * time.Millisecond)
	if _, err := os.Stat(path); err == nil {
		t.Error("FIFO file should be removed after shutdown")
	}
}

func TestShutdown_MultipleCall(t *testing.T) {
	tmpDir := setupTempDir(t)
	basePipe := filepath.Join(tmpDir, "multi_shutdown.pipe")

	_, err := InitWithBase(basePipe)
	if err != nil {
		t.Fatalf("Failed to initialize FIFO: %v", err)
	}

	// Reset sync variables
	stopOnce = sync.Once{}
	stopping = make(chan struct{})

	// Multiple calls to Shutdown should not panic
	go Shutdown()
	go Shutdown()
	go Shutdown()

	time.Sleep(100 * time.Millisecond)
}

// ================= Integration Tests =================

func TestMain_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	tmpDir := setupTempDir(t)
	basePipe := filepath.Join(tmpDir, "integration.pipe")

	// Set environment variable
	oldEnv := os.Getenv("FOCOTIMER_PIPE")
	os.Setenv("FOCOTIMER_PIPE", basePipe)
	defer os.Setenv("FOCOTIMER_PIPE", oldEnv)

	// Reset global state
	fifoPipePath = ""
	startOnce = sync.Once{}
	stopOnce = sync.Once{}
	stopping = make(chan struct{})
	wg = sync.WaitGroup{}

	// Set up timer manager
	tm := focotimer.NewTimerManager(200 * time.Millisecond)
	SetTimerManager(tm)

	// Start Main in background
	go func() {
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("Main() panicked: %v", r)
			}
		}()
		Main()
	}()

	// Wait for initialization
	time.Sleep(100 * time.Millisecond)

	// Verify FIFO was created
	if fifoPipePath == "" {
		t.Fatal("Expected fifoPipePath to be set after Main start")
	}

	if !waitForFile(fifoPipePath, 2*time.Second) {
		t.Fatal("FIFO file should exist after Main start")
	}

	// Test sending commands
	go writeToFifo(t, fifoPipePath, "start")
	time.Sleep(50 * time.Millisecond)

	if tm.Timer.Timer == nil {
		t.Error("Expected timer to be started after 'start' command")
	}

	// Test shutdown
	go writeToFifo(t, fifoPipePath, "stop")
	time.Sleep(100 * time.Millisecond)

	// Trigger shutdown
	Shutdown()
	time.Sleep(100 * time.Millisecond)
}

func TestConcurrentOperations(t *testing.T) {
	tmpDir := setupTempDir(t)
	basePipe := filepath.Join(tmpDir, "concurrent.pipe")

	path, err := InitWithBase(basePipe)
	if err != nil {
		t.Fatalf("Failed to initialize FIFO: %v", err)
	}
	defer os.Remove(path)

	tm := focotimer.NewTimerManager(1 * time.Second)
	SetTimerManager(tm)

	// Reset sync variables
	startOnce = sync.Once{}
	stopOnce = sync.Once{}
	stopping = make(chan struct{})
	wg = sync.WaitGroup{}

	// Start command handler
	go func() {
		wg.Add(1)
		defer wg.Done()
		handle_cmds()
	}()

	time.Sleep(50 * time.Millisecond)

	var testWg sync.WaitGroup

	// Concurrent operations
	operations := []func(){
		func() { TimerStart() },
		func() { TimerStop() },
		func() { TimerInc() },
		func() { TimerDec() },
		func() { Snapshot() },
		func() { Subscribe() },
		func() { output() },
	}

	// Run operations concurrently
	for _, op := range operations {
		testWg.Add(1)
		go func(operation func()) {
			defer testWg.Done()
			for i := 0; i < 10; i++ {
				operation()
				time.Sleep(time.Millisecond)
			}
		}(op)
	}

	// Also send FIFO commands concurrently
	commands := []string{"start", "stop", "inc", "dec", "gui"}
	for _, cmd := range commands {
		testWg.Add(1)
		go func(command string) {
			defer testWg.Done()
			for i := 0; i < 5; i++ {
				writeToFifo(t, path, command)
				time.Sleep(10 * time.Millisecond)
			}
		}(cmd)
	}

	testWg.Wait()

	// Cleanup
	close(stopping)
	wg.Wait()
}

// ================= Error Handling Tests =================

func TestHandleCmds_FifoError(t *testing.T) {
	// Set an invalid FIFO path
	fifoPipePath = "/nonexistent/directory/pipe"

	// Reset stopping channel
	stopping = make(chan struct{})

	// Start handler - should handle error gracefully
	done := make(chan bool)
	go func() {
		handle_cmds()
		done <- true
	}()

	// Give it time to fail and retry
	time.Sleep(200 * time.Millisecond)

	// Signal stop
	close(stopping)

	// Should exit gracefully
	select {
	case <-done:
		// Expected
	case <-time.After(2 * time.Second):
		t.Error("handle_cmds should exit when stopping channel is closed")
	}
}

func TestMkfifoUnique_PermissionError(t *testing.T) {
	// Try to create FIFO in a directory we can't write to
	_, err := mkfifoUnique("/root/test.pipe", 0666)
	if err == nil {
		t.Error("Expected error when creating FIFO in restricted directory")
	}
}

// ================= Benchmark Tests =================

func BenchmarkOutput(b *testing.B) {
	tm := focotimer.NewTimerManager(300 * time.Second)
	SetTimerManager(tm)
	fifoPipePath = "/tmp/bench.pipe"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		output()
	}
}

func BenchmarkTruncToSecond(b *testing.B) {
	durations := []time.Duration{
		1500 * time.Millisecond,
		2750 * time.Millisecond,
		-1500 * time.Millisecond,
		500 * time.Millisecond,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		truncToSecond(durations[i%len(durations)])
	}
}

func BenchmarkTimerOperations(b *testing.B) {
	tm := focotimer.NewTimerManager(1 * time.Second)
	SetTimerManager(tm)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		TimerStart()
		TimerInc()
		Snapshot()
		TimerStop()
	}
}
