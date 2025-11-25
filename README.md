# Test Runner

A terminal-based UI (TUI) application for running and managing Go unit tests interactively.

## Features

- **Two-pane interface**: Left pane for test selection, right pane for viewing test output
- **Recursive test discovery**: Automatically finds all Go tests in a directory tree
- **Parallel execution**: Run multiple tests simultaneously with configurable parallelism
- **Test filtering**: Filter tests by name with case-insensitive search
- **Output search**: Search within test output with navigation between matches
- **Persistent logs**: Test output saved to log files for later review
- **Editor integration**: Jump directly to test source code in your editor

## Installation

```bash
go install github.com/ramondeklein/test-runner@latest
```

Or build from source:

```bash
git clone https://github.com/ramondeklein/test-runner.git
cd test-runner
go build -o test-runner .
```

## Usage

```bash
# Run in current directory
./test-runner

# Run with specific test directory
./test-runner /path/to/tests

# Specify custom log directory
./test-runner --log-dir /path/to/logs /path/to/tests
```

## Keybindings

### Global
| Key | Action |
|-----|--------|
| `Tab` | Switch focus between panes |
| `q` | Quit |

### Left Pane (Test List)
| Key | Action |
|-----|--------|
| `j` / `Down` | Move cursor down |
| `k` / `Up` | Move cursor up |
| `Space` | Toggle test selection |
| `a` | Select all tests |
| `d` | Deselect all tests |
| `i` | Invert selection |
| `g` | Run selected tests (or current if none selected) |
| `t` | Stop/terminate test or remove from queue |
| `s` | Toggle sort mode (name/selection/status) |
| `r` | Toggle recursive test discovery |
| `e` | Open test in editor |
| `[` | Move current test up in list |
| `]` | Move current test down in list |
| `+` / `-` | Increase/decrease parallelism |
| `/` | Enter filter mode |

### Right Pane (Output View)
| Key | Action |
|-----|--------|
| `j` / `Down` | Scroll down |
| `k` / `Up` | Scroll up |
| `h` / `Left` | Scroll left |
| `l` / `Right` | Scroll right |
| `PgUp` / `PgDown` | Page up/down |
| `Home` | Go to beginning |
| `End` | Go to end (re-enables auto-scroll) |
| `/` | Search in output |
| `n` | Next search match |
| `N` | Previous search match |

## Test Status Icons

| Icon | Status |
|------|--------|
| (empty) | Idle/not run |
| 🍵 | Queued |
| 🏃 | Running |
| ✅ | Passed |
| ❌ | Failed |

## Log Files

Test output is saved to log files in `~/.test-runner/<hash>/` where `<hash>` is derived from the test directory path. Use `--log-dir` to specify a custom location.

Log file format: `<TestName>.<timestamp>.log`

## License

MIT
