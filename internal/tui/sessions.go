package tui

import (
	"fmt"

	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/lipgloss"

	"github.com/muhammadjihad/spock/internal/session"
)

func newSessionTable(width int) table.Model {
	columns := sessionColumns(width)
	t := table.New(
		table.WithColumns(columns),
		table.WithRows([]table.Row{}),
		table.WithFocused(false),
		table.WithHeight(10),
	)
	s := table.DefaultStyles()
	s.Header = tableHeaderStyle
	s.Selected = tableSelectedStyle
	s.Cell = tableCellStyle
	t.SetStyles(s)
	return t
}

func sessionColumns(width int) []table.Column {
	w := width - sidebarWidth - 8
	if w < 60 {
		w = 60
	}
	idW := w * 20 / 100
	connW := w * 25 / 100
	hostW := w * 25 / 100
	userW := w * 15 / 100
	startW := w - idW - connW - hostW - userW
	if startW < 10 {
		startW = 10
	}
	return []table.Column{
		{Title: "ID", Width: idW},
		{Title: "CONNECTION", Width: connW},
		{Title: "HOST", Width: hostW},
		{Title: "USER", Width: userW},
		{Title: "STARTED", Width: startW},
	}
}

func loadSessionRows() []table.Row {
	session.CleanStale()
	sessions, err := session.ListSessions()
	if err != nil {
		return nil
	}
	rows := make([]table.Row, 0, len(sessions))
	for _, s := range sessions {
		rows = append(rows, table.Row{
			s.ID,
			s.ConnectionName,
			s.Host,
			s.User,
			s.StartedAt.Format("15:04:05"),
		})
	}
	return rows
}

func sessionEmptyView() string {
	return emptyStyle.Render("No active sessions.\n\nConnect to a host from the Connections view.")
}

func selectedSessionID(t table.Model) string {
	row := t.SelectedRow()
	if len(row) == 0 {
		return ""
	}
	return row[0]
}

func selectedSessionPID(t table.Model) int {
	row := t.SelectedRow()
	if len(row) == 0 {
		return 0
	}
	id := row[0]
	sess, err := session.GetSession(id)
	if err != nil {
		return 0
	}
	return sess.PID
}

func sessionContentView(t table.Model, width, height int, focused bool) string {
	title := contentTitle.Render(fmt.Sprintf("Sessions (%d active)", len(t.Rows())))
	t.SetWidth(width)
	t.SetHeight(height - 4)
	t.SetColumns(sessionColumns(width + sidebarWidth))

	if len(t.Rows()) == 0 {
		return lipgloss.JoinVertical(lipgloss.Left, title, sessionEmptyView())
	}

	return lipgloss.JoinVertical(lipgloss.Left, title, t.View())
}
