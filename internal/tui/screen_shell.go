package tui

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/androsh7/cloud_command/pkg/agent"
	"github.com/androsh7/cloud_command/pkg/sshx"
)

const (
	maxShellLines      = 5000
	maxShellHistory    = 200
	shellInputPrompt   = "$ "
	shellEventBuffer   = 1024
	shellChromeHeight  = 4
	shellStatusOpening = "connecting"
	shellStatusRunning = "running"
	shellStatusClosed  = "closed"
	focusAll           = -1
)

// streamFilter selects which output streams the shell view shows.
type streamFilter int

const (
	streamFilterBoth streamFilter = iota
	streamFilterStdout
	streamFilterStderr
	streamFilterCount
)

// label returns the filter name shown in the header.
func (filter streamFilter) label() string {
	switch filter {
	case streamFilterStdout:
		return "stdout"
	case streamFilterStderr:
		return "stderr"
	default:
		return "stdout+stderr"
	}
}

// shellLine is one entry of the combined shell view.
type shellLine struct {
	agentName string
	text      string
	stream    sshx.Stream
	isInput   bool
	isStatus  bool
}

// hostShell is one agent's persistent remote shell and what the interface knows about it.
type hostShell struct {
	agent      *agent.Agent
	shell      *sshx.Shell
	opening    bool
	pending    int
	lastExit   int
	hasExit    bool
	queue      []string
	transcript []shellLine
	err        error
}

// status returns a short label for the host's shell.
func (host *hostShell) status() string {
	switch {
	case host.err != nil:
		return "error: " + host.err.Error()
	case host.opening:
		return shellStatusOpening
	case host.shell == nil || host.shell.IsClosed():
		return shellStatusClosed
	case host.pending > 0:
		return shellStatusRunning
	case host.hasExit:
		return fmt.Sprintf("idle (exit %d)", host.lastExit)
	default:
		return "idle"
	}
}

// shellIndicator returns the short connection state shown on the agent list, or an empty string when there is no shell.
func (model Model) shellIndicator(name string) string {
	host, found := model.shells[name]
	if !found {
		return ""
	}
	switch {
	case host.err != nil:
		return errorStyle.Render("error")
	case host.opening:
		return "connecting"
	case host.shell == nil || host.shell.IsClosed():
		return dimStyle.Render("closed")
	case host.pending > 0:
		return "running"
	default:
		return successStyle.Render("open")
	}
}

// record appends a line to the host's own transcript, keeping it bounded.
func (host *hostShell) record(line shellLine) {
	host.transcript = append(host.transcript, line)
	if len(host.transcript) > maxShellLines {
		host.transcript = host.transcript[len(host.transcript)-maxShellLines:]
	}
}

// shellState holds the shell screen: its targets, the combined view and the input line.
type shellState struct {
	targets      []*agent.Agent
	input        textinput.Model
	viewport     viewport.Model
	lines        []shellLine
	focus        int
	filter       streamFilter
	history      []string
	historyIndex int
	returnTo     screen
}

// newShellInput returns the input line used to type commands.
func newShellInput() textinput.Model {
	input := textinput.New()
	input.Prompt = shellInputPrompt
	input.Placeholder = "type a command and press enter"
	return input
}

// shellFor returns the persistent shell record for an agent, creating it when needed.
func (model *Model) shellFor(target *agent.Agent) *hostShell {
	host, found := model.shells[target.Name()]
	if !found {
		host = &hostShell{agent: target}
		model.shells[target.Name()] = host
	}
	host.agent = target
	return host
}

// openShell switches to the shell screen for the targets, connecting any shell that is not open yet.
func (model Model) openShell(targets []*agent.Agent, returnTo screen) (tea.Model, tea.Cmd) {
	model.shell.targets = targets
	model.shell.returnTo = returnTo
	model.shell.input = newShellInput()
	model.shell.lines = nil
	model.shell.focus = focusAll
	model.shell.filter = streamFilterBoth
	model.shell.historyIndex = len(model.shell.history)
	model.screen = screenShell
	model.layoutShell()
	model.setStatus(nil)
	commands := []tea.Cmd{model.shell.input.Focus(), model.spinner.Tick}
	for _, target := range targets {
		host := model.shellFor(target)
		for _, line := range host.transcript {
			model.appendShellLine(line)
		}
		if command := model.ensureShellOpen(host); command != nil {
			commands = append(commands, command)
		}
	}
	model.refreshShellView(true)
	return model, tea.Batch(commands...)
}

// ensureShellOpen starts connecting a host's shell when it is not open, returning the command that does so.
func (model *Model) ensureShellOpen(host *hostShell) tea.Cmd {
	if host.opening || (host.shell != nil && !host.shell.IsClosed()) {
		return nil
	}
	host.opening = true
	host.err = nil
	host.pending = 0
	rows, columns := model.terminalSize()
	return openShellOn(model.ctx, host.agent, model.shellEvents, rows, columns)
}

// openShellOn connects a shell to one agent and forwards its events into the shared channel.
func openShellOn(ctx context.Context, target *agent.Agent, events chan<- tea.Msg, rows int, columns int) tea.Cmd {
	return func() tea.Msg {
		sshHost, err := target.SshHost()
		if err != nil {
			return shellOpenedMsg{agentName: target.Name(), err: err}
		}
		shell, err := sshHost.OpenShell(ctx, rows, columns)
		if err != nil {
			return shellOpenedMsg{agentName: target.Name(), err: err}
		}
		go func() {
			for event := range shell.Events() {
				events <- shellEventMsg{agentName: target.Name(), event: event}
			}
		}()
		return shellOpenedMsg{agentName: target.Name(), shell: shell}
	}
}

// closeShell ends an agent's shell and forgets it.
func (model *Model) closeShell(name string) {
	host, found := model.shells[name]
	if !found {
		return
	}
	if host.shell != nil {
		_ = host.shell.Close()
	}
	delete(model.shells, name)
}

// closeAllShells ends every open shell.
func (model *Model) closeAllShells() {
	for name := range model.shells {
		model.closeShell(name)
	}
}

// layoutShell sizes the viewport for the current terminal and target count.
func (model *Model) layoutShell() {
	height := model.contentHeight() - shellChromeHeight - len(model.shell.targets)
	model.shell.viewport.Width = model.width
	model.shell.viewport.Height = max(height, 3)
	model.shell.input.Width = max(model.width-len(shellInputPrompt)-2, 20)
	rows, columns := model.terminalSize()
	for _, host := range model.shells {
		if host.shell != nil && !host.shell.IsClosed() {
			_ = host.shell.Resize(rows, columns)
		}
	}
}

// terminalSize returns the remote terminal size that fits the view once host prefixes are accounted for.
func (model Model) terminalSize() (int, int) {
	longestName := 0
	for name := range model.shells {
		longestName = max(longestName, len(name))
	}
	prefixWidth := 0
	if len(model.shells) > 1 {
		prefixWidth = longestName + len("[] ")
	}
	return max(model.shell.viewport.Height, 5), max(model.width-prefixWidth-1, 40)
}

// appendShellLine adds one line to the combined view.
func (model *Model) appendShellLine(line shellLine) {
	model.shell.lines = append(model.shell.lines, line)
	if len(model.shell.lines) > maxShellLines {
		model.shell.lines = model.shell.lines[len(model.shell.lines)-maxShellLines:]
	}
}

// focusedTargets returns the hosts the view and the input currently apply to.
func (model Model) focusedTargets() []*agent.Agent {
	if model.shell.focus >= 0 && model.shell.focus < len(model.shell.targets) {
		return []*agent.Agent{model.shell.targets[model.shell.focus]}
	}
	return model.shell.targets
}

// showsLine reports whether a line passes the current host focus and stream filter.
func (model Model) showsLine(line shellLine) bool {
	if model.shell.focus >= 0 && model.shell.focus < len(model.shell.targets) &&
		line.agentName != model.shell.targets[model.shell.focus].Name() {
		return false
	}
	if line.isInput || line.isStatus {
		return true
	}
	switch model.shell.filter {
	case streamFilterStdout:
		return line.stream == sshx.StreamStdout
	case streamFilterStderr:
		return line.stream == sshx.StreamStderr
	default:
		return true
	}
}

// renderShellLine formats one line for the viewport, prefixing the host when several hosts are shown.
func (model Model) renderShellLine(line shellLine) string {
	prefix := ""
	if model.shell.focus == focusAll && len(model.shell.targets) > 1 {
		prefix = dimStyle.Render("[" + line.agentName + "] ")
	}
	switch {
	case line.isInput:
		return prefix + labelStyle.Render(line.text)
	case line.isStatus:
		return prefix + dimStyle.Render(line.text)
	case line.stream == sshx.StreamStderr:
		return prefix + errorStyle.Render(line.text)
	default:
		return prefix + line.text
	}
}

// refreshShellView pushes the filtered lines into the viewport, following the tail when asked.
func (model *Model) refreshShellView(follow bool) {
	rendered := make([]string, 0, len(model.shell.lines))
	for _, line := range model.shell.lines {
		if model.showsLine(line) {
			rendered = append(rendered, model.renderShellLine(line))
		}
	}
	model.shell.viewport.SetContent(strings.Join(rendered, "\n"))
	if follow {
		model.shell.viewport.GotoBottom()
	}
}

// cycleFocus moves the view from all hosts to each host in turn and back.
func (model *Model) cycleFocus() {
	if len(model.shell.targets) <= 1 {
		return
	}
	model.shell.focus++
	if model.shell.focus >= len(model.shell.targets) {
		model.shell.focus = focusAll
	}
	model.refreshShellView(true)
}

// cycleFilter moves the stream filter from both to stdout to stderr and back.
func (model *Model) cycleFilter() {
	model.shell.filter = (model.shell.filter + 1) % streamFilterCount
	model.refreshShellView(true)
}

// isShellTarget reports whether an agent is shown on the shell screen right now.
func (model Model) isShellTarget(name string) bool {
	for _, target := range model.shell.targets {
		if target.Name() == name {
			return true
		}
	}
	return false
}

// updateShellKey handles a key press on the shell screen.
func (model Model) updateShellKey(message tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch message.String() {
	case "esc":
		model.notice = ""
		model.screen = model.shell.returnTo
		return model, nil
	case "ctrl+q":
		for _, target := range model.shell.targets {
			model.closeShell(target.Name())
		}
		model.notice = ""
		model.screen = model.shell.returnTo
		return model, nil
	case "enter":
		return model.submitShellLine()
	case "ctrl+x":
		return model.interruptShells()
	case "ctrl+l":
		model.shell.lines = nil
		model.refreshShellView(true)
		return model, nil
	case "tab":
		model.cycleFocus()
		return model, nil
	case "ctrl+o":
		model.cycleFilter()
		return model, nil
	case "ctrl+s":
		return model.saveShellTranscripts()
	case "up":
		model.recallHistory(-1)
		return model, nil
	case "down":
		model.recallHistory(1)
		return model, nil
	case "pgup", "pgdown", "ctrl+u", "ctrl+d", "home", "end":
		var command tea.Cmd
		model.shell.viewport, command = model.shell.viewport.Update(message)
		return model, command
	}
	var command tea.Cmd
	model.shell.input, command = model.shell.input.Update(message)
	return model, command
}

// recallHistory moves through previously sent lines.
func (model *Model) recallHistory(direction int) {
	if len(model.shell.history) == 0 {
		return
	}
	model.shell.historyIndex = min(max(model.shell.historyIndex+direction, 0), len(model.shell.history))
	if model.shell.historyIndex == len(model.shell.history) {
		model.shell.input.SetValue("")
	} else {
		model.shell.input.SetValue(model.shell.history[model.shell.historyIndex])
	}
	model.shell.input.CursorEnd()
}

// submitShellLine sends the typed line to every target shell.
func (model Model) submitShellLine() (tea.Model, tea.Cmd) {
	line := strings.TrimSpace(model.shell.input.Value())
	if line == "" {
		return model, nil
	}
	model.shell.history = append(model.shell.history, line)
	if len(model.shell.history) > maxShellHistory {
		model.shell.history = model.shell.history[len(model.shell.history)-maxShellHistory:]
	}
	model.shell.historyIndex = len(model.shell.history)
	model.shell.input.SetValue("")
	model.setStatus(nil)

	commands := []tea.Cmd{}
	for _, target := range model.focusedTargets() {
		host := model.shellFor(target)
		echo := shellLine{agentName: target.Name(), text: shellInputPrompt + line, isInput: true}
		host.record(echo)
		model.appendShellLine(echo)
		if host.shell == nil || host.shell.IsClosed() {
			host.queue = append(host.queue, line)
			if command := model.ensureShellOpen(host); command != nil {
				commands = append(commands, command)
			}
			continue
		}
		model.sendToHost(host, line)
	}
	model.refreshShellView(true)
	return model, tea.Batch(commands...)
}

// sendToHost runs one line on a host's open shell.
func (model *Model) sendToHost(host *hostShell, line string) {
	if err := host.shell.Run(line); err != nil {
		host.err = err
		return
	}
	host.pending++
}

// interruptShells sends Ctrl-C to every open shell in the current view.
func (model Model) interruptShells() (tea.Model, tea.Cmd) {
	for _, target := range model.focusedTargets() {
		host := model.shellFor(target)
		if host.shell == nil || host.shell.IsClosed() || host.pending == 0 {
			continue
		}
		if err := host.shell.Interrupt(); err != nil {
			host.err = err
		}
	}
	model.notice = "interrupt sent to running commands"
	return model, nil
}

// saveShellTranscripts writes every target's transcript to the output directory.
func (model Model) saveShellTranscripts() (tea.Model, tea.Cmd) {
	transcripts := make([]agent.Transcript, 0, len(model.shell.targets))
	for _, target := range model.shell.targets {
		host := model.shellFor(target)
		lines := make([]string, 0, len(host.transcript))
		for _, line := range host.transcript {
			lines = append(lines, line.text)
		}
		transcripts = append(transcripts, agent.Transcript{AgentName: target.Name(), Lines: lines})
	}
	savedPath, err := agent.WriteTranscripts(model.deps.Paths.OutputDirectory, transcripts, time.Now())
	if err != nil {
		model.setStatus(err)
		return model, nil
	}
	model.setStatus(nil)
	model.notice = "saved to " + savedPath
	return model, nil
}

// applyShellOpened records a connected shell and flushes anything typed while it was connecting.
func (model Model) applyShellOpened(message shellOpenedMsg) (tea.Model, tea.Cmd) {
	host, found := model.shells[message.agentName]
	if !found {
		if message.shell != nil {
			_ = message.shell.Close()
		}
		return model, nil
	}
	host.opening = false
	if message.err != nil {
		host.err = message.err
		if model.isShellTarget(message.agentName) {
			model.setStatus(fmt.Errorf("%s: %w", message.agentName, message.err))
		}
		return model, nil
	}
	host.shell = message.shell
	host.err = nil
	for _, line := range host.queue {
		model.sendToHost(host, line)
	}
	host.queue = nil
	return model, nil
}

// applyShellEvent appends streamed output and tracks command completion, then waits for the next event.
func (model Model) applyShellEvent(message shellEventMsg) (tea.Model, tea.Cmd) {
	next := waitFor(model.shellEvents)
	host, found := model.shells[message.agentName]
	if !found {
		return model, next
	}
	event := message.event
	switch {
	case event.Closed:
		host.pending = 0
		if event.Err != nil {
			host.err = event.Err
		}
		closedLine := shellLine{agentName: message.agentName, text: "[shell closed]", isInput: true}
		host.record(closedLine)
		if model.isShellTarget(message.agentName) {
			follow := model.shell.viewport.AtBottom()
			model.appendShellLine(closedLine)
			model.refreshShellView(follow)
		}
	case event.Finished:
		if host.pending == 0 {
			return model, next
		}
		host.pending--
		host.lastExit = event.ExitCode
		host.hasExit = true
		statusLine := shellLine{agentName: message.agentName, text: fmt.Sprintf("[exit %d]", event.ExitCode), isStatus: true}
		host.record(statusLine)
		if model.isShellTarget(message.agentName) {
			follow := model.shell.viewport.AtBottom()
			model.appendShellLine(statusLine)
			model.refreshShellView(follow)
		}
	default:
		outputLine := shellLine{agentName: message.agentName, text: event.Text, stream: event.Stream}
		host.record(outputLine)
		if model.isShellTarget(message.agentName) {
			follow := model.shell.viewport.AtBottom()
			model.appendShellLine(outputLine)
			model.refreshShellView(follow)
		}
	}
	return model, next
}

// viewShell renders the shell screen.
func (model Model) viewShell() string {
	var builder strings.Builder
	builder.WriteString(headerStyle.Render("Shell on " + targetNames(model.shell.targets)))
	viewLabel := "all hosts"
	if model.shell.focus != focusAll {
		viewLabel = model.shell.targets[model.shell.focus].Name() + " only"
	}
	builder.WriteString(dimStyle.Render(fmt.Sprintf("  view: %s  streams: %s", viewLabel, model.shell.filter.label())))
	builder.WriteString("\n")
	for _, target := range model.shell.targets {
		host := model.shells[target.Name()]
		status := "closed"
		if host != nil {
			status = host.status()
			if host.opening || host.pending > 0 {
				status = model.spinner.View() + " " + status
			}
		}
		builder.WriteString(labelStyle.Render(fmt.Sprintf("  %-20s", target.Name())))
		builder.WriteString(status)
		builder.WriteString("\n")
	}
	builder.WriteString(model.shell.viewport.View())
	builder.WriteString("\n")
	builder.WriteString(model.shell.input.View())
	builder.WriteString("\n")
	return builder.String()
}

// targetNames joins the names of the targets.
func targetNames(targets []*agent.Agent) string {
	names := make([]string, 0, len(targets))
	for _, target := range targets {
		names = append(names, target.Name())
	}
	return strings.Join(names, ", ")
}
