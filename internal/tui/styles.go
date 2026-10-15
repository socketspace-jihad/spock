package tui

import "github.com/charmbracelet/lipgloss"

// Catppuccin Mocha-inspired palette
var (
	colorBase     = lipgloss.Color("#1e1e2e")
	colorSurface0 = lipgloss.Color("#313244")
	colorSurface1 = lipgloss.Color("#45475a")
	colorOverlay0 = lipgloss.Color("#6c7086")
	colorText     = lipgloss.Color("#cdd6f4")
	colorSubtext  = lipgloss.Color("#a6adc8")
	colorBlue     = lipgloss.Color("#89b4fa")
	colorGreen    = lipgloss.Color("#a6e3a1")
	colorRed      = lipgloss.Color("#f38ba8")
	colorMauve    = lipgloss.Color("#cba6f7")
	colorYellow   = lipgloss.Color("#f9e2af")
	colorTeal     = lipgloss.Color("#94e2d5")
	colorPeach    = lipgloss.Color("#fab387")
)

const sidebarWidth = 22

// Sidebar styles
var (
	sidebarStyle = lipgloss.NewStyle().
			Width(sidebarWidth).
			Background(colorBase).
			Padding(1, 1)

	sidebarTitle = lipgloss.NewStyle().
			Bold(true).
			Foreground(colorMauve).
			MarginBottom(1).
			Padding(0, 1)

	sidebarItem = lipgloss.NewStyle().
			Foreground(colorSubtext).
			Padding(0, 2)

	sidebarActiveItem = lipgloss.NewStyle().
				Foreground(colorBlue).
				Bold(true).
				Padding(0, 1)

	sidebarSelectedItem = lipgloss.NewStyle().
				Foreground(colorText).
				Background(colorSurface0).
				Bold(true).
				Padding(0, 1)
)

// Content area styles
var (
	contentStyle = lipgloss.NewStyle().
			Padding(1, 2)

	contentTitle = lipgloss.NewStyle().
			Bold(true).
			Foreground(colorText).
			MarginBottom(1)

	tableHeaderStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(colorBlue).
				BorderBottom(true).
				BorderStyle(lipgloss.NormalBorder()).
				BorderForeground(colorSurface1)

	tableSelectedStyle = lipgloss.NewStyle().
				Foreground(colorText).
				Background(colorSurface0).
				Bold(true)

	tableCellStyle = lipgloss.NewStyle().
			Foreground(colorSubtext)

	emptyStyle = lipgloss.NewStyle().
			Foreground(colorOverlay0).
			Italic(true).
			Padding(2, 0)
)

// Border & layout styles
var (
	borderStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(colorSurface1)

	dividerStyle = lipgloss.NewStyle().
			Foreground(colorSurface1)
)

// Help bar styles
var (
	helpBarStyle = lipgloss.NewStyle().
			Foreground(colorOverlay0).
			Padding(0, 1)

	helpKeyStyle = lipgloss.NewStyle().
			Foreground(colorMauve).
			Bold(true)

	helpDescStyle = lipgloss.NewStyle().
			Foreground(colorOverlay0)

	helpSepStyle = lipgloss.NewStyle().
			Foreground(colorSurface1)
)

// Status message styles
var (
	statusSuccessStyle = lipgloss.NewStyle().
				Foreground(colorGreen)

	statusErrorStyle = lipgloss.NewStyle().
				Foreground(colorRed)

	statusInfoStyle = lipgloss.NewStyle().
			Foreground(colorYellow)
)

// Dialog styles
var (
	dialogStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(colorMauve).
			Padding(1, 3).
			Width(50)

	dialogTitleStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(colorText).
				MarginBottom(1)

	dialogButtonStyle = lipgloss.NewStyle().
				Foreground(colorSubtext).
				Padding(0, 2)

	dialogActiveButtonStyle = lipgloss.NewStyle().
				Foreground(colorBase).
				Background(colorBlue).
				Bold(true).
				Padding(0, 2)
)

// Header styles
var (
	headerStyle = lipgloss.NewStyle().
			Foreground(colorMauve).
			Bold(true).
			Padding(0, 1)

	headerTitleStyle = lipgloss.NewStyle().
				Foreground(colorTeal).
				Bold(true)

	headerViewStyle = lipgloss.NewStyle().
			Foreground(colorBlue).
			Bold(true)
)

func verticalDivider(height int) string {
	s := dividerStyle.Render("│")
	result := ""
	for i := 0; i < height; i++ {
		result += s + "\n"
	}
	return result
}
