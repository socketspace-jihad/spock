package tui

import (
	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/lipgloss"

	"github.com/muhammadjihad/spock/internal/keymanager"
)

func newKeyTable(width int) table.Model {
	columns := keyColumns(width)
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

func keyColumns(width int) []table.Column {
	w := width - sidebarWidth - 8
	if w < 60 {
		w = 60
	}
	nameW := w * 25 / 100
	typeW := w * 20 / 100
	fpW := w - nameW - typeW
	return []table.Column{
		{Title: "NAME", Width: nameW},
		{Title: "TYPE", Width: typeW},
		{Title: "FINGERPRINT", Width: fpW},
	}
}

func loadKeyRows() []table.Row {
	keys, err := keymanager.ListKeys()
	if err != nil {
		return nil
	}
	rows := make([]table.Row, 0, len(keys))
	for _, k := range keys {
		rows = append(rows, table.Row{
			k.Name, k.Type, k.Fingerprint,
		})
	}
	return rows
}

func keyEmptyView() string {
	return emptyStyle.Render("No keys managed yet.\n\nPress n to generate a new SSH key.")
}

func selectedKeyName(t table.Model) string {
	row := t.SelectedRow()
	if len(row) == 0 {
		return ""
	}
	return row[0]
}

func keyContentView(t table.Model, width, height int, focused bool) string {
	title := contentTitle.Render("Keys")
	t.SetWidth(width)
	t.SetHeight(height - 4)
	t.SetColumns(keyColumns(width + sidebarWidth))

	if len(t.Rows()) == 0 {
		return lipgloss.JoinVertical(lipgloss.Left, title, keyEmptyView())
	}

	return lipgloss.JoinVertical(lipgloss.Left, title, t.View())
}
