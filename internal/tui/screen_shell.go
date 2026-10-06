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
)

// hostShell is one agent's persistent remote shell and what the interface knows about it.
type hostShell struct {
	agent      *agent.Agent
	shell      *sshx.Shell
	opening    bool
	pending    int
	lastExit   int
	hasExit    bool
	queue      []string
	transcript []string
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
func (host *hostShell) record(line string) {
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
	lines        []string
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
	model.shell.historyIndex = len(model.shell.history)
	model.screen = screenShell
	model.layoutShell()
	model.setStatus(nil)
	commands := []tea.Cmd{model.shell.input.Focus(), model.spinner.Tick}
	for _, target := range targets {
		host := model.shellFor(target)
		for _, line := range host.transcript {
			model.appendShellLine(target.Name(), line)
		}
		if command := model.ensureShellOpen(host); command != nil {
			commands = append(commands, command)
		}
	}
	model.shell.viewport.GotoBottom()
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
	return openShellOn(model.ctx, host.agent, model.shellEvents)
}

// openShellOn connects a shell to one agent and forwards its events into the shared channel.
func openShellOn(ctx context.Context, target *agent.Agent, events chan<- tea.Msg) tea.Cmd {
	return func() tea.Msg {
		sshHost, err := target.SshHost()
		if err != nil {
			return shellOpenedMsg{agentName: target.Name(), err: err}
		}
		shell, err := sshHost.OpenShell(ctx)
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
}

// prefixFor returns the host label prepended to lines when several hosts share the view.
func (model Model) prefixFor(name string) string {
	if len(model.shell.targets) <= 1 {
		return ""
	}
	return dimStyle.Render("[" + name + "] ")
}

// appendShellLine adds one line to the combined view.
func (model *Model) appendShellLine(name string, line string) {
	model.shell.lines = append(model.shell.lines, model.prefixFor(name)+line)
	if len(model.shell.lines) > maxShellLines {
		model.shell.lines = model.shell.lines[len(model.shell.lines)-maxShellLines:]
	}
}

// refreshShellView pushes the combined lines into the viewport, following the tail when it was at the bottom.
func (model *Model) refreshShellView(follow bool) {
	model.shell.viewport.SetContent(strings.Join(model.shell.lines, "\n"))
	if follow {
		model.shell.viewport.GotoBottom()
	}
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
	for _, target := range model.shell.targets {
		host := model.shellFor(target)
		echo := labelStyle.Render(shellInputPrompt + line)
		host.record(shellInputPrompt + line)
		model.appendShellLine(target.Name(), echo)
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
	if _, err := host.shell.Run(line); err != nil {
		host.err = err
		return
	}
	host.pending++
}

// interruptShells sends Ctrl-C to every open target shell.
func (model Model) interruptShells() (tea.Model, tea.Cmd) {
	for _, target := range model.shell.targets {
		host := model.shellFor(target)
		if host.shell == nil || host.shell.IsClosed() {
			continue
		}
		if err := host.shell.Interrupt(); err != nil {
			host.err = err
		}
	}
	model.notice = "interrupt sent"
	return model, nil
}

// saveShellTranscripts writes every target's transcript to the output directory.
func (model Model) saveShellTranscripts() (tea.Model, tea.Cmd) {
	transcripts := make([]agent.Transcript, 0, len(model.shell.targets))
	for _, target := range model.shell.targets {
		host := model.shellFor(target)
		transcripts = append(transcripts, agent.Transcript{AgentName: target.Name(), Lines: append([]string{}, host.transcript...)})
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
		host.record("[shell closed]")
		if model.isShellTarget(message.agentName) {
			follow := model.shell.viewport.AtBottom()
			model.appendShellLine(message.agentName, dimStyle.Render("[shell closed]"))
			model.refreshShellView(follow)
		}
	case event.Finished:
		host.pending = max(host.pending-1, 0)
		host.lastExit = event.ExitCode
		host.hasExit = true
	default:
		host.record(event.Text)
		if model.isShellTarget(message.agentName) {
			follow := model.shell.viewport.AtBottom()
			model.appendShellLine(message.agentName, event.Text)
			model.refreshShellView(follow)
		}
	}
	return model, next
}

// viewShell renders the shell screen.
func (model Model) viewShell() string {
	var builder strings.Builder
	builder.WriteString(headerStyle.Render("Shell on " + targetNames(model.shell.targets)))
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
