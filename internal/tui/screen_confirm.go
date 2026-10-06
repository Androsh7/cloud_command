package tui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

// confirmAction runs when the user answers yes.
type confirmAction func(model Model) (Model, tea.Cmd)

// confirmState holds a pending yes/no question.
type confirmState struct {
	prompt   string
	returnTo screen
	onYes    confirmAction
}

// openConfirm switches to a yes/no prompt.
func (model Model) openConfirm(prompt string, returnTo screen, onYes confirmAction) Model {
	model.confirm = confirmState{prompt: prompt, returnTo: returnTo, onYes: onYes}
	model.screen = screenConfirm
	model.setStatus(nil)
	return model
}

// updateConfirmKey handles a key press on the confirm screen.
func (model Model) updateConfirmKey(message tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch message.String() {
	case "y", "Y":
		model.screen = model.confirm.returnTo
		return model.confirm.onYes(model)
	case "n", "N", "esc", "q":
		model.screen = model.confirm.returnTo
		return model, nil
	}
	return model, nil
}

// viewConfirm renders the question.
func (model Model) viewConfirm() string {
	var builder strings.Builder
	builder.WriteString(headerStyle.Render("Confirm"))
	builder.WriteString("\n\n  ")
	builder.WriteString(model.confirm.prompt)
	builder.WriteString("\n\n  ")
	builder.WriteString(errorStyle.Render("y") + " yes    " + dimStyle.Render("n") + " no\n")
	return builder.String()
}
