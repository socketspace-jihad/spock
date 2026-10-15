package tui

import "strings"

type helpEntry struct {
	key  string
	desc string
}

func helpBar(view View, focus Focus, width int) string {
	var entries []helpEntry

	if focus == FocusSidebar {
		entries = []helpEntry{
			{"↑↓", "navigate"},
			{"enter", "select"},
			{"tab", "content"},
			{"q", "quit"},
		}
	} else {
		common := []helpEntry{
			{"↑↓", "navigate"},
			{"tab", "sidebar"},
		}

		switch view {
		case ViewConnections:
			entries = append(common,
				helpEntry{"enter", "connect"},
				helpEntry{"i", "describe"},
				helpEntry{"n", "new"},
				helpEntry{"e", "edit"},
				helpEntry{"d", "delete"},
				helpEntry{"q", "quit"},
			)
		case ViewSessions:
			entries = append(common,
				helpEntry{"enter", "mirror"},
				helpEntry{"k", "kill"},
				helpEntry{"q", "quit"},
			)
		case ViewKeys:
			entries = append(common,
				helpEntry{"enter", "show"},
				helpEntry{"n", "new"},
				helpEntry{"c", "copy"},
				helpEntry{"d", "delete"},
				helpEntry{"q", "quit"},
			)
		}
	}

	var parts []string
	for _, e := range entries {
		k := helpKeyStyle.Render(e.key)
		d := helpDescStyle.Render(e.desc)
		parts = append(parts, k+" "+d)
	}

	sep := helpSepStyle.Render(" · ")
	line := strings.Join(parts, sep)

	return helpBarStyle.Width(width).Render(line)
}
