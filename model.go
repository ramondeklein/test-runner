package main

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// Pane represents which pane has focus
type Pane int

const (
	LeftPane Pane = iota
	RightPane
)

// SortMode represents the sorting mode for test list
type SortMode int

const (
	SortByName SortMode = iota
	SortBySelection
	SortByStatus
)

// Model is the main application model
type Model struct {
	tests       []*TestItem
	cursor      int
	focusedPane Pane
	runner      *TestRunner
	testDir     string
	logDir      string

	// Recursive mode (default true)
	recursive bool

	// Sort mode
	sortMode SortMode

	// Filter state
	filterMode   bool
	filterText   string
	filteredList []*TestItem

	// Output view state
	outputLines         []string
	outputScroll        int
	autoScroll          bool
	horizontalScroll    int
	currentLogFile      string    // Currently displayed log file
	currentLogTimestamp time.Time // Timestamp of currently displayed log

	// Search state (right pane)
	searchMode      bool
	searchText      string
	searchMatches   []int // Line numbers with matches
	currentMatchIdx int   // Index in searchMatches

	// Window dimensions
	width  int
	height int

	// Update ticker
	lastUpdate time.Time
}

// tickMsg is sent periodically to update the display
type tickMsg time.Time

// updateMsg is sent when test status changes
type updateMsg struct{}

// getDefaultLogDir returns the default log directory based on test directory hash
func getDefaultLogDir(testDir string) (string, error) {
	absPath, err := filepath.Abs(testDir)
	if err != nil {
		return "", err
	}

	// Create hash of the absolute path
	hash := sha256.Sum256([]byte(absPath))
	hashStr := hex.EncodeToString(hash[:8]) // Use first 8 bytes (16 hex chars)

	// Get home directory
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}

	return filepath.Join(homeDir, ".test-runner", hashStr), nil
}

// NewModel creates a new application model
func NewModel(testDir string, logDir string) (*Model, error) {
	tests, err := DiscoverTests(testDir)
	if err != nil {
		return nil, fmt.Errorf("failed to discover tests: %w", err)
	}

	items := make([]*TestItem, len(tests))
	for i, t := range tests {
		items[i] = &TestItem{
			Info:   t,
			Status: StatusIdle,
		}
	}

	// Determine log directory
	if logDir == "" {
		logDir, err = getDefaultLogDir(testDir)
		if err != nil {
			return nil, fmt.Errorf("failed to determine log directory: %w", err)
		}
	}

	// Create log directory if it doesn't exist
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create log directory: %w", err)
	}

	runner := NewTestRunner(testDir, logDir, 3) // Default parallelism

	m := &Model{
		tests:        items,
		filteredList: items,
		runner:       runner,
		testDir:      testDir,
		logDir:       logDir,
		autoScroll:   true,
		recursive:    true, // Default to recursive
		sortMode:     SortByName,
	}

	// Set the test list reference on the runner
	runner.SetTestList(&m.filteredList)

	return m, nil
}

// Init initializes the model
func (m *Model) Init() tea.Cmd {
	// Set up update callback
	m.runner.SetUpdateCallback(func() {
		// This is called from goroutines, we'll handle updates via tick
	})

	return tea.Batch(
		tickCmd(),
	)
}

// tickCmd returns a command that sends tick messages
func tickCmd() tea.Cmd {
	return tea.Tick(100*time.Millisecond, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

// Update handles messages
func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		return m.handleKey(msg)

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case tickMsg:
		m.refreshOutput()
		return m, tickCmd()

	case updateMsg:
		return m, nil
	}

	return m, nil
}

// handleKey processes keyboard input
func (m *Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Handle filter mode input (left pane)
	if m.filterMode {
		return m.handleFilterKey(msg)
	}

	// Handle search mode input (right pane)
	if m.searchMode {
		return m.handleSearchKey(msg)
	}

	key := msg.String()

	switch key {
	case "q", "ctrl+c":
		return m, tea.Quit

	case "tab":
		if m.focusedPane == LeftPane {
			m.focusedPane = RightPane
		} else {
			m.focusedPane = LeftPane
		}
		return m, nil
	}

	switch m.focusedPane {
	case LeftPane:
		return m.handleLeftPaneKey(msg)
	case RightPane:
		return m.handleRightPaneKey(msg)
	}

	return m, nil
}

// handleLeftPaneKey handles keys when left pane is focused
func (m *Model) handleLeftPaneKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()

	switch key {
	case "up", "k":
		if m.cursor > 0 {
			m.cursor--
			m.resetOutputScroll()
		}

	case "down", "j":
		if m.cursor < len(m.filteredList)-1 {
			m.cursor++
			m.resetOutputScroll()
		}

	case "/":
		m.filterMode = true
		m.filterText = ""

	case "a":
		// Select all
		for _, t := range m.filteredList {
			t.Selected = true
		}

	case "d":
		// Deselect all
		for _, t := range m.filteredList {
			t.Selected = false
		}

	case "i":
		// Invert selection
		for _, t := range m.filteredList {
			t.Selected = !t.Selected
		}

	case " ":
		// Toggle current item selection
		if len(m.filteredList) > 0 && m.cursor < len(m.filteredList) {
			m.filteredList[m.cursor].Selected = !m.filteredList[m.cursor].Selected
		}

	case "g":
		// Run selected tests (or current if none selected)
		m.runSelectedTests()

	case "t":
		// Stop selected tests (or current if none selected)
		m.stopSelectedTests()

	case "s":
		// Toggle sort mode (capital S to avoid conflict with stop)
		m.toggleSortMode()

	case "e":
		// Edit: open IDE at test function
		m.openInEditor()

	case "r":
		// Toggle recursive mode
		m.toggleRecursive()

	case "[":
		// Move current item up
		m.moveItemUp()

	case "]":
		// Move current item down
		m.moveItemDown()

	case "+", "=":
		// Increase parallelism
		m.runner.SetMaxParallel(m.runner.GetMaxParallel() + 1)

	case "-", "_":
		// Decrease parallelism
		if m.runner.GetMaxParallel() > 1 {
			m.runner.SetMaxParallel(m.runner.GetMaxParallel() - 1)
		}
	}

	return m, nil
}

// handleRightPaneKey handles keys when right pane is focused
func (m *Model) handleRightPaneKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()
	maxScroll := m.maxOutputScroll()

	switch key {
	case "up", "k":
		if m.outputScroll > 0 {
			m.outputScroll--
			m.autoScroll = false
		}

	case "down", "j":
		if m.outputScroll < maxScroll {
			m.outputScroll++
		}
		if m.outputScroll >= maxScroll {
			m.autoScroll = true
		}

	case "left", "h":
		if m.horizontalScroll > 0 {
			m.horizontalScroll--
		}

	case "right", "l":
		m.horizontalScroll++

	case "pgup":
		m.outputScroll -= m.outputHeight()
		if m.outputScroll < 0 {
			m.outputScroll = 0
		}
		m.autoScroll = false

	case "pgdown":
		m.outputScroll += m.outputHeight()
		if m.outputScroll > maxScroll {
			m.outputScroll = maxScroll
		}
		if m.outputScroll >= maxScroll {
			m.autoScroll = true
		}

	case "home":
		m.outputScroll = 0
		m.autoScroll = false

	case "end":
		m.outputScroll = maxScroll
		m.autoScroll = true

	case "/":
		// Start search mode
		m.searchMode = true
		m.searchText = ""
		m.searchMatches = nil
		m.currentMatchIdx = 0

	case "n":
		// Go to next search match
		m.goToNextMatch()

	case "N":
		// Go to previous search match
		m.goToPrevMatch()
	}

	return m, nil
}

// handleFilterKey handles keys in filter mode
func (m *Model) handleFilterKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()

	switch key {
	case "enter", "esc":
		m.filterMode = false
		m.applyFilter()

	case "backspace":
		if len(m.filterText) > 0 {
			m.filterText = m.filterText[:len(m.filterText)-1]
			m.applyFilter()
		}

	default:
		if len(key) == 1 {
			m.filterText += key
			m.applyFilter()
		}
	}

	return m, nil
}

// handleSearchKey handles keys in search mode (right pane)
func (m *Model) handleSearchKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()

	switch key {
	case "enter":
		m.searchMode = false
		m.performSearch()
		m.goToNextMatch()

	case "esc":
		m.searchMode = false

	case "backspace":
		if len(m.searchText) > 0 {
			m.searchText = m.searchText[:len(m.searchText)-1]
		}

	default:
		if len(key) == 1 {
			m.searchText += key
		}
	}

	return m, nil
}

// performSearch searches for text in output lines (case insensitive)
func (m *Model) performSearch() {
	m.searchMatches = nil
	m.currentMatchIdx = -1

	if m.searchText == "" {
		return
	}

	searchLower := strings.ToLower(m.searchText)
	for i, line := range m.outputLines {
		if strings.Contains(strings.ToLower(line), searchLower) {
			m.searchMatches = append(m.searchMatches, i)
		}
	}
}

// goToNextMatch scrolls to the next search match
func (m *Model) goToNextMatch() {
	if len(m.searchMatches) == 0 {
		return
	}

	m.currentMatchIdx++
	if m.currentMatchIdx >= len(m.searchMatches) {
		m.currentMatchIdx = 0
	}

	// Scroll to the match
	m.outputScroll = m.searchMatches[m.currentMatchIdx]
	m.autoScroll = false
}

// goToPrevMatch scrolls to the previous search match
func (m *Model) goToPrevMatch() {
	if len(m.searchMatches) == 0 {
		return
	}

	m.currentMatchIdx--
	if m.currentMatchIdx < 0 {
		m.currentMatchIdx = len(m.searchMatches) - 1
	}

	// Scroll to the match
	m.outputScroll = m.searchMatches[m.currentMatchIdx]
	m.autoScroll = false
}

// applyFilter filters the test list based on filter text
func (m *Model) applyFilter() {
	if m.filterText == "" {
		m.filteredList = m.tests
	} else {
		m.filteredList = nil
		filter := strings.ToLower(m.filterText)
		for _, t := range m.tests {
			if strings.Contains(strings.ToLower(t.Info.Name), filter) {
				m.filteredList = append(m.filteredList, t)
			}
		}
	}

	// Adjust cursor if needed
	if m.cursor >= len(m.filteredList) {
		m.cursor = len(m.filteredList) - 1
	}
	if m.cursor < 0 {
		m.cursor = 0
	}
}

// runSelectedTests queues selected tests for execution
func (m *Model) runSelectedTests() {
	hasSelected := false
	for _, t := range m.filteredList {
		if t.Selected {
			hasSelected = true
			m.runner.QueueTest(t)
		}
	}

	// If none selected, run current item
	if !hasSelected && len(m.filteredList) > 0 && m.cursor < len(m.filteredList) {
		m.runner.QueueTest(m.filteredList[m.cursor])
	}
}

// stopSelectedTests stops selected tests
func (m *Model) stopSelectedTests() {
	hasSelected := false
	for _, t := range m.filteredList {
		if t.Selected {
			hasSelected = true
			m.runner.StopTest(t)
		}
	}

	// If none selected, stop current item
	if !hasSelected && len(m.filteredList) > 0 && m.cursor < len(m.filteredList) {
		m.runner.StopTest(m.filteredList[m.cursor])
	}
}

// moveItemUp moves the current item up in the list
func (m *Model) moveItemUp() {
	if m.cursor <= 0 || len(m.filteredList) == 0 {
		return
	}

	m.filteredList[m.cursor], m.filteredList[m.cursor-1] = m.filteredList[m.cursor-1], m.filteredList[m.cursor]
	m.cursor--
}

// moveItemDown moves the current item down in the list
func (m *Model) moveItemDown() {
	if m.cursor >= len(m.filteredList)-1 || len(m.filteredList) == 0 {
		return
	}
	m.filteredList[m.cursor], m.filteredList[m.cursor+1] = m.filteredList[m.cursor+1], m.filteredList[m.cursor]
	m.cursor++
}

// toggleSortMode cycles through sort modes
func (m *Model) toggleSortMode() {
	m.sortMode = (m.sortMode + 1) % 3
	m.applySorting()
}

// applySorting sorts the filtered list based on current sort mode
func (m *Model) applySorting() {
	// Remember current item
	var currentItem *TestItem
	if len(m.filteredList) > 0 && m.cursor < len(m.filteredList) {
		currentItem = m.filteredList[m.cursor]
	}

	switch m.sortMode {
	case SortByName:
		sort.Slice(m.filteredList, func(i, j int) bool {
			return m.filteredList[i].Info.Name < m.filteredList[j].Info.Name
		})
	case SortBySelection:
		sort.Slice(m.filteredList, func(i, j int) bool {
			if m.filteredList[i].Selected != m.filteredList[j].Selected {
				return m.filteredList[i].Selected
			}
			return m.filteredList[i].Info.Name < m.filteredList[j].Info.Name
		})
	case SortByStatus:
		sort.Slice(m.filteredList, func(i, j int) bool {
			if m.filteredList[i].Status != m.filteredList[j].Status {
				return m.filteredList[i].Status > m.filteredList[j].Status
			}
			return m.filteredList[i].Info.Name < m.filteredList[j].Info.Name
		})
	}

	// Restore cursor position to current item
	if currentItem != nil {
		for i, item := range m.filteredList {
			if item == currentItem {
				m.cursor = i
				break
			}
		}
	}
}

// toggleRecursive toggles recursive test discovery and refreshes the list
func (m *Model) toggleRecursive() {
	m.recursive = !m.recursive
	m.rediscoverTests()
}

// rediscoverTests re-runs test discovery with current settings
func (m *Model) rediscoverTests() {
	tests, err := DiscoverTestsRecursive(m.testDir, m.recursive)
	if err != nil {
		return
	}

	items := make([]*TestItem, len(tests))
	for i, t := range tests {
		items[i] = &TestItem{
			Info:   t,
			Status: StatusIdle,
		}
	}

	m.tests = items
	m.applyFilter()
	m.applySorting()
}

// openInEditor opens the current test in the IDE
func (m *Model) openInEditor() {
	if len(m.filteredList) == 0 || m.cursor >= len(m.filteredList) {
		return
	}

	item := m.filteredList[m.cursor]
	file := item.Info.File
	line := item.Info.Line

	// Try common editors in order of preference
	// Format: editor +line file
	editors := []struct {
		cmd  string
		args []string
	}{
		{"code", []string{"--goto", fmt.Sprintf("%s:%d", file, line)}},
		{"cursor", []string{"--goto", fmt.Sprintf("%s:%d", file, line)}},
		{"vim", []string{fmt.Sprintf("+%d", line), file}},
		{"nvim", []string{fmt.Sprintf("+%d", line), file}},
		{"nano", []string{fmt.Sprintf("+%d", line), file}},
	}

	// Check EDITOR environment variable first
	if editor := os.Getenv("EDITOR"); editor != "" {
		cmd := exec.Command(editor, fmt.Sprintf("+%d", line), file)
		cmd.Start()
		return
	}

	// Try each editor
	for _, e := range editors {
		if _, err := exec.LookPath(e.cmd); err == nil {
			cmd := exec.Command(e.cmd, e.args...)
			cmd.Start()
			return
		}
	}
}

// findMostRecentLogFile finds the most recent log file for a test in the log directory
func (m *Model) findMostRecentLogFile(testName string) (string, time.Time) {
	pattern := filepath.Join(m.logDir, testName+".*.log")
	matches, err := filepath.Glob(pattern)
	if err != nil || len(matches) == 0 {
		return "", time.Time{}
	}

	var mostRecent string
	var mostRecentTime time.Time

	for _, match := range matches {
		info, err := os.Stat(match)
		if err != nil {
			continue
		}
		if info.ModTime().After(mostRecentTime) {
			mostRecentTime = info.ModTime()
			mostRecent = match
		}
	}

	return mostRecent, mostRecentTime
}

// refreshOutput reloads the output file content
func (m *Model) refreshOutput() {
	if len(m.filteredList) == 0 || m.cursor >= len(m.filteredList) {
		m.outputLines = nil
		m.currentLogFile = ""
		m.currentLogTimestamp = time.Time{}
		return
	}

	item := m.filteredList[m.cursor]
	logFile := item.LogFile
	var logTimestamp time.Time

	// If test hasn't run yet, try to find most recent log file
	if logFile == "" {
		logFile, logTimestamp = m.findMostRecentLogFile(item.Info.Name)
	}

	if logFile == "" {
		m.outputLines = nil
		m.currentLogFile = ""
		m.currentLogTimestamp = time.Time{}
		return
	}

	file, err := os.Open(logFile)
	if err != nil {
		m.outputLines = nil
		m.currentLogFile = ""
		m.currentLogTimestamp = time.Time{}
		return
	}
	defer file.Close()

	// Get file mod time if we don't have a timestamp yet
	if logTimestamp.IsZero() {
		if info, err := file.Stat(); err == nil {
			logTimestamp = info.ModTime()
		}
	}

	m.currentLogFile = logFile
	m.currentLogTimestamp = logTimestamp

	var lines []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}

	m.outputLines = lines

	if m.autoScroll {
		m.outputScroll = m.maxOutputScroll()
	}
}

// resetOutputScroll resets output scroll when changing selection
func (m *Model) resetOutputScroll() {
	m.autoScroll = true
	m.horizontalScroll = 0
	m.refreshOutput()
}

// outputHeight returns the height available for output
func (m *Model) outputHeight() int {
	return m.height - 3 // Account for borders and status bar
}

// maxOutputScroll returns the maximum scroll position
func (m *Model) maxOutputScroll() int {
	max := len(m.outputLines) - m.outputHeight()
	if max < 0 {
		return 0
	}
	return max
}
