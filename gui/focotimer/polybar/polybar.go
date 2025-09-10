package polybar

import (
	"bufio"
	"errors"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	focotimer "github.com/d093w1z/focotimer/api"
)

var (
	fifoPipePath string

	mu                sync.RWMutex
	guiToggleCallback func()

	timerMu   sync.Mutex
	startOnce sync.Once
	stopOnce  sync.Once
	wg        sync.WaitGroup
	stopping  = make(chan struct{})

	timerManager *focotimer.TimerManager
)

// --- TimerManager injection ---

// SetTimerManager lets the application provide a shared TimerManager instance.
// Safe to call before or after Init().
func SetTimerManager(tm *focotimer.TimerManager) {
	timerMu.Lock()
	defer timerMu.Unlock()
	timerManager = tm
}

// getTimerManager safely returns the current TimerManager or nil.
func getTimerManager() *focotimer.TimerManager {
	timerMu.Lock()
	defer timerMu.Unlock()
	return timerManager
}

// --- Polybar setup ---

func Init() {
	base := os.Getenv("FOCOTIMER_PIPE")
	if base == "" {
		base = "/tmp/focotimer.pipe"
	}
	path, err := InitWithBase(base)
	if err != nil {
		log.Fatalf("polybar.Init: %v", err)
	}
	log.Printf("FIFO created at %q", path)
}

func InitWithBase(base string) (string, error) {
	abs := base
	if !filepath.IsAbs(abs) {
		abs = filepath.Join(os.TempDir(), base)
	}

	path, err := mkfifoUnique(abs, 0666)
	if err != nil {
		return "", err
	}
	fifoPipePath = path
	return path, nil
}

func mkfifoUnique(base string, mode os.FileMode) (string, error) {
	// Add PID to make it unique per process
	pid := os.Getpid()

	for i := 0; i < 1000; i++ {
		var path string
		if i == 0 {
			path = fmt.Sprintf("%s.%d", base, pid)
		} else {
			path = fmt.Sprintf("%s.%d.%d", base, pid, i)
		}

		err := syscall.Mkfifo(path, uint32(mode.Perm()))
		if err == nil {
			return path, nil
		}
		if errors.Is(err, os.ErrExist) || err == syscall.EEXIST {
			fi, statErr := os.Lstat(path)
			if statErr != nil {
				continue
			}
			if (fi.Mode() & os.ModeNamedPipe) != 0 {
				// Check if the FIFO is actually usable (not in use by another process)
				if canUseFifo(path) {
					return path, nil
				}
			}
			continue
		}
		return "", fmt.Errorf("mkfifo %q: %w", path, err)
	}
	return "", fmt.Errorf("unable to allocate unique FIFO for base %q after many attempts", base)
}

// canUseFifo checks if we can actually use this FIFO (not locked by another process)
func canUseFifo(path string) bool {
	// Try to open for writing with O_NONBLOCK to test availability
	file, err := os.OpenFile(path, os.O_WRONLY|syscall.O_NONBLOCK, 0)
	if err != nil {
		return false
	}
	file.Close()
	return true
}

// --- Handlers ---

func AddHandler(f func()) {
	mu.Lock()
	guiToggleCallback = f
	mu.Unlock()
}

func Main() {
	if fifoPipePath == "" {
		Init()
	}

	startOnce.Do(func() {
		wg.Add(1)
		go func() {
			defer wg.Done()
			handle_cmds()
		}()
	})

	// Set up signal handling BEFORE starting the main loop
	sigc := make(chan os.Signal, 2) // Increased buffer size
	signal.Notify(sigc, syscall.SIGINT, syscall.SIGTERM)

	// Don't defer signal.Stop here - we want signals throughout the loop
	defer func() {
		signal.Stop(sigc)
		close(sigc)
	}()

	t := time.NewTicker(1 * time.Second)
	defer t.Stop()

	// Defensive: check timer manager before use
	if tm := getTimerManager(); tm != nil {
		Subscribe()
		// TimerStart()
		// tm.Timer.AddHandler(func() { log.Println("Timer finished!") })
	} else {
		log.Println("polybar.Main: no TimerManager set, timer disabled")
	}

	log.Println("polybar.Main: starting main loop")

	for {
		select {
		case <-t.C:
			fmt.Println(output())
		case sig := <-sigc:
			log.Printf("polybar.Main: received signal %v, shutting down", sig)
			Shutdown()
			return
		case <-stopping:
			log.Println("polybar.Main: stopping channel triggered")
			return
		}
	}
}

func Shutdown() {
	log.Println("polybar.Shutdown: initiating shutdown")
	stopOnce.Do(func() {
		close(stopping)
		if fifoPipePath != "" {
			log.Printf("polybar.Shutdown: removing FIFO %q", fifoPipePath)
			if err := os.Remove(fifoPipePath); err != nil && !errors.Is(err, os.ErrNotExist) {
				log.Printf("warning: removing FIFO %q: %v", fifoPipePath, err)
			}
		}
	})
	log.Println("polybar.Shutdown: waiting for goroutines")
	wg.Wait()
	log.Println("polybar.Shutdown: complete")
}

func FifoPath() string { return fifoPipePath }

// --- Internal command loop ---

func handle_cmds() {
	log.Println("polybar.handle_cmds: starting command handler")
	defer log.Println("polybar.handle_cmds: command handler stopped")

	for {
		select {
		case <-stopping:
			log.Println("polybar.handle_cmds: stopping signal received")
			return
		default:
		}

		log.Printf("polybar.handle_cmds: opening FIFO %q", fifoPipePath)
		file, err := os.OpenFile(fifoPipePath, os.O_RDONLY, os.ModeNamedPipe)
		if err != nil {
			log.Printf("polybar.handle_cmds: open FIFO error: %v", err)
			// Check if we're shutting down
			select {
			case <-stopping:
				return
			case <-time.After(time.Second):
				continue
			}
		}

		log.Println("polybar.handle_cmds: FIFO opened, reading commands")
		scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			cmd := scanner.Text()
			log.Printf("polybar.handle_cmds: received command: %q", cmd)
			switch cmd {
			case "start":
				TimerStart()
			case "gui":
				mu.RLock()
				cb := guiToggleCallback
				mu.RUnlock()
				if cb != nil {
					cb()
				}
			case "inc":
				TimerInc()
			case "dec":
				TimerDec()
			case "stop":
				TimerStop()
			default:
				log.Printf("polybar.handle_cmds: unknown command: %q", cmd)
			}
		}

		if err := scanner.Err(); err != nil {
			log.Printf("polybar.handle_cmds: scanner error: %v", err)
		}

		log.Println("polybar.handle_cmds: closing FIFO")
		_ = file.Close()

		// Small delay before reopening to prevent tight loops
		select {
		case <-stopping:
			return
		case <-time.After(100 * time.Millisecond):
		}
	}
}

func polybarActionButton(button string, action string) string {
	lbl := button
	if len(lbl) > 0 && lbl[len(lbl)-1] == '\n' {
		lbl = lbl[:len(lbl)-1]
	}
	return fmt.Sprintf("%%{A:%s:} %s %%{A}", action, lbl)
}

func pipeCommand(cmd string) string {
	return fmt.Sprintf("echo '%s' > %s", cmd, fifoPipePath)
}

// --- Output helpers ---

func output() string {
	dur, rem := timerSnapshot()
	timestring := fmt.Sprintf("%s : %s", truncToSecond(dur), truncToSecond(rem))

	return polybarActionButton("[-]", pipeCommand("dec")) +
		polybarActionButton(timestring, pipeCommand("gui")) +
		polybarActionButton("[+]", pipeCommand("inc"))
}

// --- Timer wrappers (null-safe) ---

func TimerStart() {
	if tm := getTimerManager(); tm != nil {
		tm.Start()
	}
}
func TimerStop() {
	if tm := getTimerManager(); tm != nil {
		tm.Stop()
	}
}
func TimerInc() {
	if tm := getTimerManager(); tm != nil {
		tm.Inc()
	}
}
func TimerDec() {
	if tm := getTimerManager(); tm != nil {
		tm.Dec()
	}
}
func Subscribe() <-chan time.Duration {
	if tm := getTimerManager(); tm != nil {
		return tm.Subscribe()
	}
	return nil
}
func Snapshot() time.Duration {
	if tm := getTimerManager(); tm != nil {
		return tm.Snapshot()
	}
	return 0
}

func timerSnapshot() (time.Duration, time.Duration) {
	if tm := getTimerManager(); tm != nil {
		d := tm.Timer.Duration
		r := tm.Snapshot()
		return d, r
	}
	return 0, 0
}

func truncToSecond(d time.Duration) time.Duration {
	if d < 0 {
		return -((-d).Truncate(time.Second))
	}
	return d.Truncate(time.Second)
}
