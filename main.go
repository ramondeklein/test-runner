package main

import (
	"flag"
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
)

func main() {
	// Parse command line flags
	logDir := flag.String("log-dir", "", "Directory for log files (default: ~/.test-runner/<hash>)")
	testTimeout := flag.Duration("test-timeout", 0, "Timeout for each test (default: 30m)")
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: %s [options] [test-directory]\n\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "Options:\n")
		flag.PrintDefaults()
	}
	flag.Parse()

	// Get test directory from remaining args or use current directory
	testDir := "."
	if flag.NArg() > 0 {
		testDir = flag.Arg(0)
	}

	// Verify directory exists
	info, err := os.Stat(testDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: cannot access directory %s: %v\n", testDir, err)
		os.Exit(1)
	}
	if !info.IsDir() {
		fmt.Fprintf(os.Stderr, "Error: %s is not a directory\n", testDir)
		os.Exit(1)
	}

	// Create the model
	model, err := NewModel(testDir, *logDir, *testTimeout)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	// Create and run the program
	p := tea.NewProgram(model, tea.WithAltScreen())

	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error running program: %v\n", err)
		os.Exit(1)
	}
}
