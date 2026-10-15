package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/lipgloss"

	"github.com/muhammadjihad/spock/internal/store"
)

func newConnTable(width int) table.Model {
	columns := connColumns(width)
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

func connColumns(width int) []table.Column {
	w := width - sidebarWidth - 8
	if w < 80 {
		w = 80
	}
	nameW := w * 14 / 100
	hostW := w * 18 / 100
	portW := 5
	osW := w * 18 / 100
	cpuW := w * 6 / 100
	memW := w * 8 / 100
	diskW := w * 7 / 100
	gpuW := w * 16 / 100
	lastW := w - nameW - hostW - portW - osW - cpuW - memW - diskW - gpuW
	if lastW < 8 {
		lastW = 8
	}
	return []table.Column{
		{Title: "NAME", Width: nameW},
		{Title: "HOST", Width: hostW},
		{Title: "PORT", Width: portW},
		{Title: "OS", Width: osW},
		{Title: "CPU", Width: cpuW},
		{Title: "MEM", Width: memW},
		{Title: "DISK", Width: diskW},
		{Title: "GPU", Width: gpuW},
		{Title: "LAST USED", Width: lastW},
	}
}

func loadConnRows() []table.Row {
	conns, err := store.List()
	if err != nil {
		return nil
	}
	rows := make([]table.Row, 0, len(conns))
	for _, c := range conns {
		osInfo := "-"
		cpu := "-"
		mem := "-"
		disk := "-"
		gpu := "-"

		if c.NodeInfo != nil {
			if c.NodeInfo.OS != "" && c.NodeInfo.OS != "Unknown" {
				osInfo = truncate(c.NodeInfo.OS, 20)
			}
			if c.NodeInfo.CPUCores != "" && c.NodeInfo.CPUCores != "Unknown" {
				cpu = c.NodeInfo.CPUCores + "c"
			}
			mem = parseMemShort(c.NodeInfo.MemInfo)
			if c.NodeInfo.DiskTotal != "" && c.NodeInfo.DiskTotal != "Unknown" {
				disk = c.NodeInfo.DiskTotal
			}
			gpu = parseGPUShort(c.NodeInfo.GPU)
		}

		last := "-"
		if c.LastConnected != nil {
			last = c.LastConnected.Format("Jan 02 15:04")
		}
		rows = append(rows, table.Row{
			c.Name, c.Host, fmt.Sprintf("%d", c.Port),
			osInfo, cpu, mem, disk, gpu, last,
		})
	}
	return rows
}

// parseMemShort extracts total memory from free -h output like "Mem:  3.8Gi ..."
func parseMemShort(memInfo string) string {
	if memInfo == "" || memInfo == "Unavailable" {
		return "-"
	}
	for _, line := range strings.Split(memInfo, "\n") {
		if strings.HasPrefix(line, "Mem:") {
			fields := strings.Fields(line)
			if len(fields) >= 2 {
				return fields[1]
			}
		}
	}
	return "-"
}

// parseGPUShort returns a short GPU label for the table column.
func parseGPUShort(gpu string) string {
	if gpu == "" || gpu == "None" {
		return "None"
	}
	first := strings.TrimSpace(strings.Split(gpu, "\n")[0])
	if first == "" {
		return "None"
	}
	// Count how many GPUs (lines)
	lines := 0
	for _, l := range strings.Split(gpu, "\n") {
		if strings.TrimSpace(l) != "" {
			lines++
		}
	}
	label := truncate(first, 18)
	if lines > 1 {
		label += fmt.Sprintf(" +%d", lines-1)
	}
	return label
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max-1] + "…"
}

func connEmptyView() string {
	return emptyStyle.Render("No connections saved yet.\n\nPress n to add a new connection.")
}

func selectedConnName(t table.Model) string {
	row := t.SelectedRow()
	if len(row) == 0 {
		return ""
	}
	return row[0]
}

func connContentView(t table.Model, width, height int, focused bool) string {
	title := contentTitle.Render("Connections")
	t.SetWidth(width)
	t.SetHeight(height - 4)
	t.SetColumns(connColumns(width + sidebarWidth))

	if len(t.Rows()) == 0 {
		return lipgloss.JoinVertical(lipgloss.Left, title, connEmptyView())
	}

	return lipgloss.JoinVertical(lipgloss.Left, title, t.View())
}
