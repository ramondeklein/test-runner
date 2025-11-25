# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

This is a Go TUI application for running and managing Go unit tests interactively. The application displays a two-pane interface: a left pane for test selection and a right pane for viewing test output.

## Build and Run Commands

```bash
# Initialize module (if not done)
go mod init test-runner

# Build
go build -o test-runner .

# Run (defaults to current directory)
./test-runner

# Run with specific test directory
./test-runner /path/to/tests
```

## Technology Requirements

- Language: Go
- UI: Terminal-based (TUI) with two panes and status bar

## Version control

- Project is managed using GIT.
- Build artifacts are ignored.
- All files ending with `.log` are ignored.

## Application Specifications

### General Behavior
- Two-pane layout: left (test list), right (output), plus status bar
- Tab key switches focus between panes; left pane has initial focus
- Discovers Go unit tests from current directory (or CLI-specified directory)

### Left Pane (Test List)
- Recursive: `r` toggle recursive integration test listing (default is recursive)
- Navigation: cursor up/down, `k` (up), `j` (down)
- Filter: `/` followed by filter text (case insensitive)
- Selection: `a` (select all), `d` (deselect all), `i` (invert selection)
- Execution: `g` (run selected/current), `t` (terminate test or remove from queue)
- Reorder: `[` (move up), `]` (move down)
- Parallelism: `+`/`-` to adjust `N_parallel` (default: 3)
- Sorting: `s` (toggles between sorted by name, selection, running state)
- Edit: `e` (open the IDE and open the file and set cursor to the start of the test function)
- Name is always shown without the `Test` prefix
- Test status prefixes:
  - (empty) idle/initial
  - 🍵 queued
  - 🏃 running
  - ✅ succeeded
  - ❌ failed
- Timer shows queue wait time or run time

### Right Pane (Output View)
- Shows output of selected test. If test hasn't run yet, it loads the most recent test log file.
- Output header contains test name. If it shows a log file from a previous run it will show the timestamp of that run.
- Log files: `<test-name>.<timestamp>.log`
- Default: auto-scroll to end (tail mode)
- Navigation: arrows, page up/down, home/end, `h` (left), `k` (up), `j` (down), `l` (right)
- Use `/` to search for a specific text (case insensitive). `n` will find the next occurance.
- Manual scroll disables auto-scroll; End key re-enables it
- Log files should be stored in `~/.test-runner/<hash of test-directory>/`, unless a log folder is set using `--log-dir`.

### Status Bar
- Displays: `N_parallel` value, running test count, queued test count colluegue

### Execution Rules
- Max `N_parallel` tests run simultaneously
- Queued tests start in queue order
- Increasing parallelism starts more queued tests immediately
- Decreasing parallelism lets running tests finish
