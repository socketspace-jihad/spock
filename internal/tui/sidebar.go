package tui

import "strings"

type View int

const (
	ViewConnections View = iota
	ViewSessions
	ViewKeys
)

var viewLabels = []string{
	"Connections",
	"Sessions",
	"Keys",
}

var viewIcons = []string{
	"󰣀",
	"󱘖",
	"󰌆",
}

type Sidebar struct {
	cursor int
	items  []string
	icons  []string
}

func NewSidebar() Sidebar {
	return Sidebar{
		cursor: 0,
		items:  viewLabels,
		icons:  viewIcons,
	}
}

func (s *Sidebar) Up() {
	if s.cursor > 0 {
		s.cursor--
	}
}

func (s *Sidebar) Down() {
	if s.cursor < len(s.items)-1 {
		s.cursor++
	}
}

func (s *Sidebar) Selected() View {
	return View(s.cursor)
}

func (s Sidebar) View(focused bool, height int) string {
	var b strings.Builder

	title := sidebarTitle.Render("  SPOCK")
	b.WriteString(title)
	b.WriteString("\n\n")

	for i, item := range s.items {
		icon := s.icons[i]
		label := icon + " " + item

		var line string
		if i == s.cursor && focused {
			line = sidebarSelectedItem.Render(" ▸ " + label + " ")
		} else if i == s.cursor {
			line = sidebarActiveItem.Render(" ▸ " + label)
		} else {
			line = sidebarItem.Render("   " + label)
		}
		b.WriteString(line)
		b.WriteString("\n")
	}

	// Pad to fill height
	lines := strings.Count(b.String(), "\n")
	for i := lines; i < height-2; i++ {
		b.WriteString("\n")
	}

	return sidebarStyle.Height(height).Render(b.String())
}
