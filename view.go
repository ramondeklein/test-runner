package main

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
)

var (
	// Colors
	focusedBorderColor   = lipgloss.Color("205")
	unfocusedBorderColor = lipgloss.Color("240")
	selectedColor        = lipgloss.Color("170")
	cursorColor          = lipgloss.Color("212")
	statusBarColor       = lipgloss.Color("236")
	statusTextColor      = lipgloss.Color("252")

	// Status icons
	statusIcons = map[TestStatus]string{
		StatusIdle:    "   ",
		StatusQueued:  "🍵 ",
		StatusRunning: "🏃 ",
		StatusPassed:  "✅ ",
		StatusFailed:  "❌ ",
	}
)

// View renders the UI
func (m *Model) View() string {
	if m.width == 0 || m.height == 0 {
		return "Loading..."
	}

	leftWidth := m.width / 3
	rightWidth := m.width - leftWidth - 1 // -1 for separator
	contentHeight := m.height - 2         // -2 for status bar

	leftPane := m.renderLeftPane(leftWidth, contentHeight)
	rightPane := m.renderRightPane(rightWidth, contentHeight)
	statusBar := m.renderStatusBar()

	// Combine panes side by side
	content := lipgloss.JoinHorizontal(lipgloss.Top, leftPane, rightPane)

	return lipgloss.JoinVertical(lipgloss.Left, content, statusBar)
}

// renderLeftPane renders the test list pane
func (m *Model) renderLeftPane(width, height int) string {
	borderColor := unfocusedBorderColor
	if m.focusedPane == LeftPane {
		borderColor = focusedBorderColor
	}

	style := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(borderColor).
		Width(width - 2).
		Height(height - 2)

	// Build content
	var content strings.Builder

	// Filter line
	if m.filterMode {
		content.WriteString(fmt.Sprintf("Filter: %s█\n", m.filterText))
	} else if m.filterText != "" {
		content.WriteString(fmt.Sprintf("Filter: %s\n", m.filterText))
	}

	// Calculate visible range
	listHeight := height - 3 // Account for border and potential filter line
	if m.filterMode || m.filterText != "" {
		listHeight--
	}

	startIdx := 0
	if m.cursor >= listHeight {
		startIdx = m.cursor - listHeight + 1
	}

	endIdx := startIdx + listHeight
	if endIdx > len(m.filteredList) {
		endIdx = len(m.filteredList)
	}

	// Render visible items
	for i := startIdx; i < endIdx; i++ {
		item := m.filteredList[i]

		// Build the line
		var line strings.Builder

		// Selection marker
		if item.Selected {
			line.WriteString("●")
		} else {
			line.WriteString(" ")
		}

		// Status icon
		line.WriteString(statusIcons[item.Status])

		// Test name (without "Test" prefix)
		name := strings.TrimPrefix(item.Info.Name, "Test")
		if item.Info.Package != "" {
			name = item.Info.Package + "/" + name
		}

		// Timer
		var timer string
		if item.Status == StatusQueued || item.Status == StatusRunning ||
			item.Status == StatusPassed || item.Status == StatusFailed {
			dur := item.Duration()
			timer = fmt.Sprintf(" %s", formatDuration(dur))
		}

		// Truncate name if needed
		maxNameWidth := width - 10 - len(timer) // Account for markers and timer
		if len(name) > maxNameWidth && maxNameWidth > 3 {
			name = name[:maxNameWidth-3] + "..."
		}

		line.WriteString(name)
		line.WriteString(timer)

		// Apply cursor highlighting
		lineStr := line.String()
		if i == m.cursor {
			lineStr = lipgloss.NewStyle().
				Background(cursorColor).
				Foreground(lipgloss.Color("0")).
				Render(lineStr)
		} else if item.Selected {
			lineStr = lipgloss.NewStyle().
				Foreground(selectedColor).
				Render(lineStr)
		}

		content.WriteString(lineStr)
		if i < endIdx-1 {
			content.WriteString("\n")
		}
	}

	// Fill remaining space
	linesWritten := endIdx - startIdx
	for i := linesWritten; i < listHeight; i++ {
		content.WriteString("\n")
	}

	return style.Render(content.String())
}

// renderRightPane renders the output pane
func (m *Model) renderRightPane(width, height int) string {
	borderColor := unfocusedBorderColor
	if m.focusedPane == RightPane {
		borderColor = focusedBorderColor
	}

	style := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(borderColor).
		Width(width - 2).
		Height(height - 2)

	var content strings.Builder

	// Show current test info
	if len(m.filteredList) > 0 && m.cursor < len(m.filteredList) {
		item := m.filteredList[m.cursor]
		testName := strings.TrimPrefix(item.Info.Name, "Test")
		header := fmt.Sprintf("Output: %s", testName)

		// Add timestamp if showing a previous run's log
		if item.LogFile == "" && !m.currentLogTimestamp.IsZero() {
			header += fmt.Sprintf(" (from %s)", m.currentLogTimestamp.Format("2006-01-02 15:04:05"))
		}

		if len(header) > width-4 {
			header = header[:width-7] + "..."
		}
		content.WriteString(lipgloss.NewStyle().Bold(true).Render(header))
		content.WriteString("\n")
		content.WriteString(strings.Repeat("─", width-4))
		content.WriteString("\n")
	}

	// Calculate visible range
	outputHeight := height - 5 // Account for header and borders
	if outputHeight < 1 {
		outputHeight = 1
	}

	startLine := m.outputScroll
	endLine := startLine + outputHeight
	if endLine > len(m.outputLines) {
		endLine = len(m.outputLines)
	}

	lineWidth := width - 4

	// Render visible lines
	linesRendered := 0
	for i := startLine; i < endLine; i++ {
		line := m.outputLines[i]

		// Apply horizontal scroll
		if m.horizontalScroll > 0 && len(line) > m.horizontalScroll {
			line = line[m.horizontalScroll:]
		} else if m.horizontalScroll > 0 {
			line = ""
		}

		// Truncate to width
		if len(line) > lineWidth {
			line = line[:lineWidth]
		}

		content.WriteString(line)
		linesRendered++
		if i < endLine-1 {
			content.WriteString("\n")
		}
	}

	// Fill remaining space
	for i := linesRendered; i < outputHeight; i++ {
		content.WriteString("\n")
	}

	// Search mode input or scroll indicator
	if m.searchMode {
		content.WriteString("\n")
		searchPrompt := fmt.Sprintf("Search: %s█", m.searchText)
		content.WriteString(lipgloss.NewStyle().Bold(true).Render(searchPrompt))
	} else {
		// Scroll indicator and search info
		var infoItems []string

		if len(m.outputLines) > outputHeight {
			scrollInfo := fmt.Sprintf("[%d/%d]", m.outputScroll+1, len(m.outputLines))
			if m.autoScroll {
				scrollInfo += " (auto)"
			}
			infoItems = append(infoItems, scrollInfo)
		}

		if m.searchText != "" && len(m.searchMatches) > 0 {
			matchInfo := fmt.Sprintf("'%s' %d/%d", m.searchText, m.currentMatchIdx+1, len(m.searchMatches))
			infoItems = append(infoItems, matchInfo)
		} else if m.searchText != "" {
			infoItems = append(infoItems, fmt.Sprintf("'%s' not found", m.searchText))
		}

		if len(infoItems) > 0 {
			content.WriteString("\n")
			content.WriteString(lipgloss.NewStyle().Faint(true).Render(" " + strings.Join(infoItems, " │ ")))
		}
	}

	return style.Render(content.String())
}

// renderStatusBar renders the status bar
func (m *Model) renderStatusBar() string {
	style := lipgloss.NewStyle().
		Background(statusBarColor).
		Foreground(statusTextColor).
		Width(m.width).
		Padding(0, 1)

	// Left side: controls help
	leftInfo := "q:quit │ g:go │ t:stop │ s:sort │ e:edit │ r:rec │ +/-:par │ /:filter"

	// Right side: status info with recursive indicator
	recursiveIndicator := "on"
	if !m.recursive {
		recursiveIndicator = "off"
	}

	sortModeStr := ""
	switch m.sortMode {
	case SortByName:
		sortModeStr = "name"
	case SortBySelection:
		sortModeStr = "sel"
	case SortByStatus:
		sortModeStr = "status"
	}

	rightInfo := fmt.Sprintf("Sort:%s │ Rec:%s │ Par:%d │ Run:%d │ Queue:%d",
		sortModeStr,
		recursiveIndicator,
		m.runner.GetMaxParallel(),
		m.runner.GetRunningCount(),
		m.runner.GetQueuedCount())

	// Calculate spacing
	spacing := m.width - len(leftInfo) - len(rightInfo) - 4
	if spacing < 1 {
		spacing = 1
	}

	statusText := leftInfo + strings.Repeat(" ", spacing) + rightInfo

	return style.Render(statusText)
}

// formatDuration formats a duration for display
func formatDuration(d time.Duration) string {
	if d < time.Second {
		return fmt.Sprintf("%dms", d.Milliseconds())
	}
	if d < time.Minute {
		return fmt.Sprintf("%.1fs", d.Seconds())
	}
	return fmt.Sprintf("%dm%ds", int(d.Minutes()), int(d.Seconds())%60)
}
