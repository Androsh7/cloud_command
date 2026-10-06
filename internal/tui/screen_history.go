package tui

import (
	"fmt"
	"log/slog"
	"strings"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/androsh7/cloud_command/pkg/history"
)

const historyScriptPreviewLength = 60

// historyState holds the recorded commands and jobs shown on the history screens.
type historyState struct {
	entries  []history.Entry
	cursor   int
	loading  bool
	viewport viewport.Model
}

// recordHistory appends an entry and returns its id, or an empty string when there is no history store.
func (model *Model) recordHistory(entry history.Entry) string {
	if model.deps.History == nil {
		return ""
	}
	recorded, err := model.deps.History.Record(entry)
	if err != nil {
		slog.Warn("recording history failed", "error", err)
		return ""
	}
	return recorded.Id
}

// finishHistory stores the exit code of an earlier entry.
func (model *Model) finishHistory(id string, exitCode int) {
	if model.deps.History == nil || id == "" {
		return
	}
	if err := model.deps.History.SetResult(id, &exitCode, ""); err != nil {
		slog.Warn("updating history failed", "error", err)
	}
}

// openHistory switches to the history list and loads it.
func (model Model) openHistory() (tea.Model, tea.Cmd) {
	model.screen = screenHistory
	model.history.loading = true
	model.setStatus(nil)
	return model, tea.Batch(model.spinner.Tick, model.loadHistory())
}

// loadHistory reads every entry from the history store.
func (model Model) loadHistory() tea.Cmd {
	store := model.deps.History
	return func() tea.Msg {
		if store == nil {
			return historyLoadedMsg{entries: []history.Entry{}}
		}
		entries, err := store.List()
		return historyLoadedMsg{entries: entries, err: err}
	}
}

// applyHistoryLoaded stores the loaded entries.
func (model Model) applyHistoryLoaded(message historyLoadedMsg) (tea.Model, tea.Cmd) {
	model.history.loading = false
	if message.err != nil {
		model.setStatus(message.err)
		return model, nil
	}
	model.history.entries = message.entries
	if model.history.cursor >= len(model.history.entries) {
		model.history.cursor = max(len(model.history.entries)-1, 0)
	}
	return model, nil
}

// updateHistoryKey handles a key press on the history list.
func (model Model) updateHistoryKey(message tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch message.String() {
	case "esc", "q":
		model.screen = screenAgents
		return model, nil
	case "up", "k":
		if model.history.cursor > 0 {
			model.history.cursor--
		}
	case "down", "j":
		if model.history.cursor < len(model.history.entries)-1 {
			model.history.cursor++
		}
	case "r":
		if !model.history.loading {
			model.history.loading = true
			return model, tea.Batch(model.spinner.Tick, model.loadHistory())
		}
	case "enter":
		if model.history.cursor < len(model.history.entries) {
			model.history.viewport = viewport.New(model.width, max(model.contentHeight()-8, 3))
			model.history.viewport.SetContent(renderHistoryEntry(model.history.entries[model.history.cursor]))
			model.screen = screenHistoryDetail
		}
	}
	return model, nil
}

// updateHistoryDetailKey handles a key press on one history entry.
func (model Model) updateHistoryDetailKey(message tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch message.String() {
	case "esc", "q", "enter":
		model.screen = screenHistory
		return model, nil
	}
	var command tea.Cmd
	model.history.viewport, command = model.history.viewport.Update(message)
	return model, command
}

// resultLabel returns the exit code, the error or a dash for an entry.
func resultLabel(entry history.Entry) string {
	switch {
	case entry.Error != "":
		return "error"
	case entry.ExitCode != nil:
		return fmt.Sprintf("exit %d", *entry.ExitCode)
	default:
		return "-"
	}
}

// visibleRows returns the slice of entries that fits around the cursor.
func (model Model) visibleRows() (int, int) {
	rows := max(model.contentHeight()-4, 3)
	start := 0
	if model.history.cursor >= rows {
		start = model.history.cursor - rows + 1
	}
	return start, min(start+rows, len(model.history.entries))
}

// viewHistory renders the history list.
func (model Model) viewHistory() string {
	var builder strings.Builder
	builder.WriteString(headerStyle.Render("History"))
	builder.WriteString("\n")
	if len(model.history.entries) == 0 && !model.history.loading {
		builder.WriteString(dimStyle.Render("  Nothing recorded yet."))
		builder.WriteString("\n")
		return builder.String()
	}
	builder.WriteString(dimStyle.Render(fmt.Sprintf("  %-19s %-16s %-7s %-4s %-5s %-9s %s", "TIME", "AGENT", "KIND", "SRC", "SUDO", "RESULT", "SCRIPT")))
	builder.WriteString("\n")
	start, end := model.visibleRows()
	for index := start; index < end; index++ {
		entry := model.history.entries[index]
		sudo := "no"
		if entry.Sudo {
			sudo = "yes"
		}
		script := previewScript(entry.Script)
		if len(script) > historyScriptPreviewLength {
			script = script[:historyScriptPreviewLength-3] + "..."
		}
		line := fmt.Sprintf("%-19s %-16s %-7s %-4s %-5s %-9s %s",
			entry.Time.Local().Format(timestampLayout), entry.Agent, entry.Kind, entry.Source, sudo, resultLabel(entry), script)
		builder.WriteString(model.renderRow(line, index == model.history.cursor))
	}
	if end < len(model.history.entries) {
		builder.WriteString(dimStyle.Render(fmt.Sprintf("  ... %d more", len(model.history.entries)-end)))
		builder.WriteString("\n")
	}
	return builder.String()
}

// renderHistoryEntry formats one entry with its full script.
func renderHistoryEntry(entry history.Entry) string {
	var builder strings.Builder
	rows := [][2]string{
		{"Time", entry.Time.Local().Format(timestampLayout)},
		{"Agent", entry.Agent},
		{"Kind", string(entry.Kind)},
		{"Source", string(entry.Source)},
		{"Sudo", fmt.Sprintf("%t", entry.Sudo)},
		{"Result", resultLabel(entry)},
	}
	if entry.JobId != "" {
		rows = append(rows, [2]string{"Job id", entry.JobId})
	}
	if entry.Error != "" {
		rows = append(rows, [2]string{"Error", entry.Error})
	}
	for _, row := range rows {
		builder.WriteString(labelStyle.Render(fmt.Sprintf("  %-8s", row[0])))
		builder.WriteString(row[1])
		builder.WriteString("\n")
	}
	builder.WriteString(labelStyle.Render("  Script"))
	builder.WriteString("\n")
	for _, line := range strings.Split(strings.TrimRight(entry.Script, "\n"), "\n") {
		builder.WriteString("    ")
		builder.WriteString(line)
		builder.WriteString("\n")
	}
	return builder.String()
}

// viewHistoryDetail renders one entry.
func (model Model) viewHistoryDetail() string {
	var builder strings.Builder
	builder.WriteString(headerStyle.Render("History entry"))
	builder.WriteString("\n")
	builder.WriteString(model.history.viewport.View())
	builder.WriteString("\n")
	return builder.String()
}
