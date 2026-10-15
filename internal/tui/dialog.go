package tui

import (
	"github.com/charmbracelet/lipgloss"
)

type Dialog struct {
	title   string
	message string
	confirm bool // true = confirmed
	done    bool
}

func NewConfirmDialog(title, message string) *Dialog {
	return &Dialog{title: title, message: message}
}

func (d *Dialog) HandleKey(key string) {
	switch key {
	case "y", "Y", "enter":
		d.confirm = true
		d.done = true
	case "n", "N", "esc":
		d.confirm = false
		d.done = true
	}
}

func (d Dialog) View(width int) string {
	w := 50
	if w > width-4 {
		w = width - 4
	}

	style := dialogStyle.Width(w)
	title := dialogTitleStyle.Render(d.title)
	msg := lipgloss.NewStyle().Foreground(colorSubtext).Render(d.message)

	yes := dialogActiveButtonStyle.Render(" Yes (y) ")
	no := dialogButtonStyle.Render(" No (n) ")
	buttons := lipgloss.JoinHorizontal(lipgloss.Center, yes, "  ", no)

	content := lipgloss.JoinVertical(lipgloss.Center,
		title,
		"",
		msg,
		"",
		buttons,
	)

	return style.Render(content)
}
