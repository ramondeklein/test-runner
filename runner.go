package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"
)

// TestStatus represents the current state of a test
type TestStatus int

const (
	StatusIdle TestStatus = iota
	StatusQueued
	StatusRunning
	StatusPassed
	StatusFailed
)

var defaultTestTimeout = 30 * time.Minute

// TestItem represents a test in the list with its current state
type TestItem struct {
	Info       TestInfo
	Status     TestStatus
	Selected   bool
	LogFile    string
	QueuedAt   time.Time
	StartedAt  time.Time
	FinishedAt time.Time
	cancel     context.CancelFunc
	mu         sync.Mutex
}

// Duration returns the appropriate duration based on status
func (t *TestItem) Duration() time.Duration {
	t.mu.Lock()
	defer t.mu.Unlock()

	switch t.Status {
	case StatusQueued:
		return time.Since(t.QueuedAt)
	case StatusRunning:
		return time.Since(t.StartedAt)
	case StatusPassed, StatusFailed:
		return t.FinishedAt.Sub(t.StartedAt)
	default:
		return 0
	}
}

// TestRunner manages test execution with parallelism control
type TestRunner struct {
	testDir     string
	logDir      string
	maxParallel int
	testTimeout time.Duration
	running     int
	tests       *[]*TestItem // Reference to the test list
	mu          sync.Mutex
	onUpdate    func()
}

// NewTestRunner creates a new test runner
func NewTestRunner(testDir string, logDir string, maxParallel int, testTimeout time.Duration) *TestRunner {
	if testTimeout <= 0 {
		testTimeout = defaultTestTimeout
	}
	return &TestRunner{
		testDir:     testDir,
		logDir:      logDir,
		maxParallel: maxParallel,
		testTimeout: testTimeout,
	}
}

// SetUpdateCallback sets the callback for status updates
func (r *TestRunner) SetUpdateCallback(cb func()) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.onUpdate = cb
}

// SetTestList sets the reference to the test list
func (r *TestRunner) SetTestList(tests *[]*TestItem) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.tests = tests
}

// SetMaxParallel updates the max parallel limit
func (r *TestRunner) SetMaxParallel(n int) {
	r.mu.Lock()
	r.maxParallel = n
	r.mu.Unlock()

	// Try to start more tests if parallelism increased
	r.tryStartNext()
}

// GetMaxParallel returns the current max parallel limit
func (r *TestRunner) GetMaxParallel() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.maxParallel
}

// GetRunningCount returns the number of running tests
func (r *TestRunner) GetRunningCount() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.running
}

// GetQueuedCount returns the number of queued tests
func (r *TestRunner) GetQueuedCount() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.tests == nil {
		return 0
	}
	count := 0
	for _, item := range *r.tests {
		item.mu.Lock()
		if item.Status == StatusQueued {
			count++
		}
		item.mu.Unlock()
	}
	return count
}

// QueueTest marks a test as queued for execution
func (r *TestRunner) QueueTest(item *TestItem) {
	item.mu.Lock()
	if item.Status == StatusRunning || item.Status == StatusQueued {
		item.mu.Unlock()
		return
	}

	item.Status = StatusQueued
	item.QueuedAt = time.Now()

	// Create log file path in log directory
	timestamp := time.Now().Format("20060102-150405")
	logFileName := fmt.Sprintf("%s.%s.log", item.Info.Name, timestamp)
	item.LogFile = filepath.Join(r.logDir, logFileName)
	item.mu.Unlock()

	r.notifyUpdate()
	r.tryStartNext()
}

// StopTest stops a running or queued test
func (r *TestRunner) StopTest(item *TestItem) {
	item.mu.Lock()
	defer item.mu.Unlock()

	switch item.Status {
	case StatusQueued:
		// Just reset status to idle
		item.Status = StatusIdle
		r.notifyUpdate()

	case StatusRunning:
		// Cancel the running test
		if item.cancel != nil {
			item.cancel()
		}
	}
}

// tryStartNext attempts to start the next queued test from the list
func (r *TestRunner) tryStartNext() {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.tests == nil {
		return
	}

	// Scan the list in order and start queued tests
	for r.running < r.maxParallel {
		var nextItem *TestItem
		for _, item := range *r.tests {
			item.mu.Lock()
			if item.Status == StatusQueued {
				nextItem = item
				item.mu.Unlock()
				break
			}
			item.mu.Unlock()
		}

		if nextItem == nil {
			break
		}

		r.running++
		go r.runTest(nextItem)
	}
}

// runTest executes a single test
func (r *TestRunner) runTest(item *TestItem) {
	ctx, cancel := context.WithCancel(context.Background())

	item.mu.Lock()
	item.Status = StatusRunning
	item.StartedAt = time.Now()
	item.cancel = cancel
	item.mu.Unlock()

	r.notifyUpdate()

	// Create log file
	logFile, err := os.Create(item.LogFile)
	if err != nil {
		item.mu.Lock()
		item.Status = StatusFailed
		item.FinishedAt = time.Now()
		item.mu.Unlock()
		r.testFinished()
		return
	}
	defer logFile.Close()

	// Determine the package path for go test
	pkgPath := "./..."
	if item.Info.Package != "" {
		pkgPath = "./" + item.Info.Package
	} else {
		pkgPath = "."
	}

	// Run the test
	cmd := exec.CommandContext(ctx, "go", "test", "-timeout", r.testTimeout.String(), "-v", "-run", fmt.Sprintf("^%s$", item.Info.Name), pkgPath)
	cmd.Dir = r.testDir
	cmd.Stdout = logFile
	cmd.Stderr = logFile

	err = cmd.Run()

	item.mu.Lock()
	item.FinishedAt = time.Now()
	if ctx.Err() == context.Canceled {
		item.Status = StatusFailed
	} else if err != nil {
		item.Status = StatusFailed
	} else {
		item.Status = StatusPassed
	}
	item.cancel = nil
	item.mu.Unlock()

	r.testFinished()
}

// testFinished is called when a test completes
func (r *TestRunner) testFinished() {
	r.mu.Lock()
	r.running--
	r.mu.Unlock()

	r.notifyUpdate()
	r.tryStartNext()
}

// notifyUpdate calls the update callback if set
func (r *TestRunner) notifyUpdate() {
	r.mu.Lock()
	cb := r.onUpdate
	r.mu.Unlock()

	if cb != nil {
		cb()
	}
}
