package tui

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"

	"github.com/muhammadjihad/spock/internal/config"
	"github.com/muhammadjihad/spock/internal/keymanager"
	"github.com/muhammadjihad/spock/internal/keyring"
	"github.com/muhammadjihad/spock/internal/sshclient"
	"github.com/muhammadjihad/spock/internal/store"
)

type Focus int

const (
	FocusSidebar Focus = iota
	FocusContent
)

// Messages
type (
	tabOpenedMsg   struct{ err error }
	statusClearMsg struct{}
	refreshMsg     struct{}
	describeDoneMsg struct {
		name string
		info *store.CachedNodeInfo
		err  error
	}
)

type App struct {
	width, height int

	sidebar Sidebar
	focus   Focus
	view    View

	connTable    table.Model
	sessionTable table.Model
	keyTable     table.Model

	form       *huh.Form
	formAction FormAction
	formVals   *formValues

	dialog *Dialog

	// Describe modal
	describeVP      viewport.Model
	describeVisible bool
	describeName    string
	describeFetching bool

	statusMsg   string
	statusStyle lipgloss.Style

	quitting bool
}

func NewApp() App {
	return App{
		sidebar:      NewSidebar(),
		focus:        FocusSidebar,
		view:         ViewConnections,
		connTable:    newConnTable(80),
		sessionTable: newSessionTable(80),
		keyTable:     newKeyTable(80),
	}
}

func (a App) Init() tea.Cmd {
	return tea.Batch(tea.EnterAltScreen, a.refreshData())
}

func (a *App) refreshData() tea.Cmd {
	return func() tea.Msg {
		return refreshMsg{}
	}
}

func (a *App) doRefresh() {
	a.connTable.SetRows(loadConnRows())
	a.sessionTable.SetRows(loadSessionRows())
	a.keyTable.SetRows(loadKeyRows())
}

func (a App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	case tea.WindowSizeMsg:
		a.width = msg.Width
		a.height = msg.Height
		contentH := a.height - 4
		a.connTable.SetHeight(contentH - 4)
		a.sessionTable.SetHeight(contentH - 4)
		a.keyTable.SetHeight(contentH - 4)
		return a, nil

	case refreshMsg:
		a.doRefresh()
		return a, nil

	case statusClearMsg:
		a.statusMsg = ""
		return a, nil

	case tabOpenedMsg:
		if msg.err != nil {
			a.statusMsg = msg.err.Error()
			a.statusStyle = statusErrorStyle
		} else {
			a.statusMsg = "Opened in new tab"
			a.statusStyle = statusSuccessStyle
		}
		return a, a.clearStatusAfter(3 * time.Second)

	case describeDoneMsg:
		a.describeFetching = false
		if msg.err != nil {
			a.statusMsg = "Describe failed: " + msg.err.Error()
			a.statusStyle = statusErrorStyle
			return a, a.clearStatusAfter(5 * time.Second)
		}
		// Cache it
		store.SaveNodeInfo(msg.name, msg.info)
		a.doRefresh()
		a.showDescribeModal(msg.name, msg.info)
		return a, nil

	case tea.KeyMsg:
		return a.handleKey(msg)
	}

	// Forward non-key messages to form if active (huh needs these for internal state)
	if a.form != nil {
		form, cmd := a.form.Update(msg)
		if f, ok := form.(*huh.Form); ok {
			a.form = f
			if f.State == huh.StateCompleted {
				cmd2 := a.handleFormSubmit()
				if a.form == nil {
					// Form fully done (not replaced by another form)
					a.formAction = FormNone
				}
				if cmd2 != nil {
					return a, tea.Batch(cmd2, a.refreshData())
				}
				return a, a.refreshData()
			}
		}
		return a, cmd
	}

	return a, nil
}

func (a *App) clearStatusAfter(d time.Duration) tea.Cmd {
	return tea.Tick(d, func(time.Time) tea.Msg {
		return statusClearMsg{}
	})
}

func (a App) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()

	// Dialog takes priority
	if a.dialog != nil {
		a.dialog.HandleKey(key)
		if a.dialog.done {
			if a.dialog.confirm {
				cmd := a.executeDialogAction()
				a.dialog = nil
				return a, cmd
			}
			a.dialog = nil
		}
		return a, nil
	}

	// Describe modal — scrollable, Esc/q to close, R to refresh
	if a.describeVisible {
		switch key {
		case "esc", "q":
			a.describeVisible = false
			return a, nil
		case "R":
			// Force re-fetch
			conn, err := store.Get(a.describeName)
			if err == nil {
				a.describeVisible = false
				a.describeFetching = true
				a.statusMsg = "Refreshing system info from " + a.describeName + "..."
				a.statusStyle = statusInfoStyle
				return a, a.fetchDescribe(a.describeName, conn)
			}
			return a, nil
		default:
			var cmd tea.Cmd
			a.describeVP, cmd = a.describeVP.Update(msg)
			return a, cmd
		}
	}

	// Form takes priority — forward all keys to form except Esc
	if a.form != nil {
		if key == "esc" {
			a.form = nil
			a.formAction = FormNone
			return a, nil
		}
		form, cmd := a.form.Update(msg)
		if f, ok := form.(*huh.Form); ok {
			a.form = f
			if f.State == huh.StateCompleted {
				cmd2 := a.handleFormSubmit()
				if a.form == nil {
					a.formAction = FormNone
				}
				if cmd2 != nil {
					return a, tea.Batch(cmd2, a.refreshData())
				}
				return a, a.refreshData()
			}
		}
		return a, cmd
	}

	// Global keys
	switch key {
	case "q", "ctrl+c":
		a.quitting = true
		return a, tea.Quit

	case "tab":
		if a.focus == FocusSidebar {
			a.focus = FocusContent
			a.setTableFocus(true)
		} else {
			a.focus = FocusSidebar
			a.setTableFocus(false)
		}
		return a, nil
	}

	if a.focus == FocusSidebar {
		return a.handleSidebarKey(key)
	}
	return a.handleContentKey(msg, key)
}

func (a *App) setTableFocus(focused bool) {
	switch a.view {
	case ViewConnections:
		a.connTable.Focus()
	case ViewSessions:
		a.sessionTable.Focus()
	case ViewKeys:
		a.keyTable.Focus()
	}
	if !focused {
		a.connTable.Blur()
		a.sessionTable.Blur()
		a.keyTable.Blur()
	}
}

func (a App) handleSidebarKey(key string) (tea.Model, tea.Cmd) {
	switch key {
	case "up", "k":
		a.sidebar.Up()
		a.view = a.sidebar.Selected()
		return a, a.refreshData()
	case "down", "j":
		a.sidebar.Down()
		a.view = a.sidebar.Selected()
		return a, a.refreshData()
	case "enter":
		a.focus = FocusContent
		a.setTableFocus(true)
		return a, nil
	}
	return a, nil
}

func (a App) handleContentKey(msg tea.KeyMsg, key string) (tea.Model, tea.Cmd) {
	switch a.view {
	case ViewConnections:
		return a.handleConnKey(msg, key)
	case ViewSessions:
		return a.handleSessionKey(msg, key)
	case ViewKeys:
		return a.handleKeyViewKey(msg, key)
	}
	return a, nil
}

func (a App) handleConnKey(msg tea.KeyMsg, key string) (tea.Model, tea.Cmd) {
	switch key {
	case "enter", "c":
		name := selectedConnName(a.connTable)
		if name != "" {
			conn, err := store.Get(name)
			if err != nil {
				return a, nil
			}
			a.formAction = FormSelectUser
			a.formVals = &formValues{connName: name}
			a.form = newUserSelectForm(conn, a.formVals)
			return a, a.form.Init()
		}
	case "i":
		name := selectedConnName(a.connTable)
		if name == "" {
			return a, nil
		}
		conn, err := store.Get(name)
		if err != nil {
			return a, nil
		}
		// If cached, show modal immediately
		if conn.NodeInfo != nil {
			a.showDescribeModal(name, conn.NodeInfo)
			return a, nil
		}
		// Otherwise, fetch via SSH in background
		a.describeFetching = true
		a.statusMsg = "Fetching system info from " + name + "..."
		a.statusStyle = statusInfoStyle
		return a, a.fetchDescribe(name, conn)
	case "n":
		a.formAction = FormAddConn
		a.formVals = &formValues{port: "22"}
		a.form = newConnForm(a.formVals)
		return a, a.form.Init()
	case "e":
		name := selectedConnName(a.connTable)
		if name != "" {
			conn, err := store.Get(name)
			if err == nil {
				a.formAction = FormEditConn
				a.formVals = &formValues{
					name:     conn.Name,
					host:     conn.Host,
					user:     conn.User,
					port:     fmt.Sprintf("%d", conn.Port),
					key:      conn.KeyName,
					jumpHost: conn.JumpHost,
				}
				a.form = newConnForm(a.formVals)
				return a, a.form.Init()
			}
		}
	case "d":
		name := selectedConnName(a.connTable)
		if name != "" {
			a.dialog = NewConfirmDialog(
				"Delete Connection",
				fmt.Sprintf("Remove %q? This cannot be undone.", name),
			)
		}
		return a, nil
	default:
		var cmd tea.Cmd
		a.connTable, cmd = a.connTable.Update(msg)
		return a, cmd
	}
	return a, nil
}

func (a App) handleSessionKey(msg tea.KeyMsg, key string) (tea.Model, tea.Cmd) {
	switch key {
	case "enter", "m":
		id := selectedSessionID(a.sessionTable)
		if id != "" {
			return a, a.openInNewTab("session", "mirror", id)
		}
	case "k":
		id := selectedSessionID(a.sessionTable)
		if id != "" {
			a.dialog = NewConfirmDialog(
				"Kill Session",
				fmt.Sprintf("Terminate session %q?", id),
			)
		}
		return a, nil
	case "r":
		a.doRefresh()
		return a, nil
	default:
		var cmd tea.Cmd
		a.sessionTable, cmd = a.sessionTable.Update(msg)
		return a, cmd
	}
	return a, nil
}

func (a App) handleKeyViewKey(msg tea.KeyMsg, key string) (tea.Model, tea.Cmd) {
	switch key {
	case "enter":
		name := selectedKeyName(a.keyTable)
		if name != "" {
			info, err := keymanager.GetInfo(name)
			if err == nil {
				a.statusMsg = fmt.Sprintf("%s  %s", info.Fingerprint, info.Type)
				a.statusStyle = statusInfoStyle
				return a, a.clearStatusAfter(5 * time.Second)
			}
		}
	case "n":
		a.formAction = FormCreateKey
		a.formVals = &formValues{keyType: "ed25519"}
		a.form = newKeyForm(a.formVals)
		return a, a.form.Init()
	case "c":
		name := selectedKeyName(a.keyTable)
		if name != "" {
			pub, err := keymanager.PublicKeyString(name)
			if err == nil {
				if copyToClipboard(pub) {
					a.statusMsg = "Public key copied to clipboard"
					a.statusStyle = statusSuccessStyle
				} else {
					a.statusMsg = "Clipboard unavailable"
					a.statusStyle = statusErrorStyle
				}
				return a, a.clearStatusAfter(3 * time.Second)
			}
		}
	case "d":
		name := selectedKeyName(a.keyTable)
		if name != "" {
			a.dialog = NewConfirmDialog(
				"Delete Key",
				fmt.Sprintf("Remove key %q? This cannot be undone.", name),
			)
		}
		return a, nil
	default:
		var cmd tea.Cmd
		a.keyTable, cmd = a.keyTable.Update(msg)
		return a, cmd
	}
	return a, nil
}

func (a *App) executeDialogAction() tea.Cmd {
	switch a.view {
	case ViewConnections:
		name := selectedConnName(a.connTable)
		if name != "" {
			store.Remove(name)
			a.statusMsg = fmt.Sprintf("Connection %q removed", name)
			a.statusStyle = statusSuccessStyle
			return tea.Batch(a.refreshData(), a.clearStatusAfter(3*time.Second))
		}
	case ViewSessions:
		id := selectedSessionID(a.sessionTable)
		if id != "" {
			pid := selectedSessionPID(a.sessionTable)
			if pid > 0 {
				if p, err := os.FindProcess(pid); err == nil {
					p.Signal(os.Kill)
				}
			}
			a.statusMsg = fmt.Sprintf("Session %q killed", id)
			a.statusStyle = statusSuccessStyle
			return tea.Batch(a.refreshData(), a.clearStatusAfter(3*time.Second))
		}
	case ViewKeys:
		name := selectedKeyName(a.keyTable)
		if name != "" {
			keymanager.Delete(name)
			a.statusMsg = fmt.Sprintf("Key %q removed", name)
			a.statusStyle = statusSuccessStyle
			return tea.Batch(a.refreshData(), a.clearStatusAfter(3*time.Second))
		}
	}
	return nil
}

func (a *App) handleFormSubmit() tea.Cmd {
	var err error
	switch a.formAction {
	case FormAddConn:
		err = submitConnForm(a.formVals, false)
		if err == nil {
			a.statusMsg = fmt.Sprintf("Connection %q added", a.formVals.name)
			a.statusStyle = statusSuccessStyle
		}
	case FormEditConn:
		err = submitConnForm(a.formVals, true)
		if err == nil {
			a.statusMsg = fmt.Sprintf("Connection %q updated", a.formVals.name)
			a.statusStyle = statusSuccessStyle
		}
	case FormCreateKey:
		err = submitKeyForm(a.formVals)
		if err == nil {
			a.statusMsg = fmt.Sprintf("Key %q created", a.formVals.name)
			a.statusStyle = statusSuccessStyle
		}
	case FormSelectUser:
		return a.handleUserSelected()
	}
	if err != nil {
		a.statusMsg = err.Error()
		a.statusStyle = statusErrorStyle
	}
	a.form = nil
	return nil
}

func (a *App) handleUserSelected() tea.Cmd {
	selected := a.formVals.selectedUser
	connName := a.formVals.connName

	if selected == "__custom__" {
		// Replace form with custom user input — keep formAction as FormSelectUser
		a.form = newCustomUserForm(a.formVals)
		return a.form.Init()
	}

	if selected == "" {
		a.form = nil
		return nil
	}

	// Save the user to the connection's user list if it's new
	conn, err := store.Get(connName)
	if err == nil {
		found := selected == conn.User
		for _, u := range conn.Users {
			if u == selected {
				found = true
				break
			}
		}
		if !found {
			conn.Users = append(conn.Users, selected)
			store.Update(conn)
		}
	}

	a.form = nil
	// Open terminal tab with selected user
	return a.openInNewTab("ssh", "to", connName, "--user", selected)
}

func (a *App) showDescribeModal(name string, info *store.CachedNodeInfo) {
	content := formatDescribeContent(name, info)
	w := a.width - 12
	h := a.height - 8
	if w < 40 {
		w = 40
	}
	if h < 10 {
		h = 10
	}
	vp := viewport.New(w, h)
	vp.SetContent(content)
	a.describeVP = vp
	a.describeVisible = true
	a.describeName = name
}

func (a *App) fetchDescribe(name string, conn *store.Connection) tea.Cmd {
	return func() tea.Msg {
		cfg, err := config.Load()
		if err != nil {
			return describeDoneMsg{name: name, err: err}
		}
		password, _ := keyring.GetPassword(name)
		opts := sshclient.ConnectOptions{
			Host:     conn.Host,
			Port:     conn.Port,
			User:     conn.User,
			KeyName:  conn.KeyName,
			KeyPath:  conn.KeyPath,
			Password: password,
		}
		nodeInfo, err := sshclient.DescribeNode(opts, cfg)
		if err != nil {
			return describeDoneMsg{name: name, err: err}
		}
		cached := &store.CachedNodeInfo{
			Hostname: nodeInfo.Hostname,
			OS:       nodeInfo.OS,
			Kernel:   nodeInfo.Kernel,
			CPUModel: nodeInfo.CPUModel,
			CPUCores: nodeInfo.CPUCores,
			MemInfo:  nodeInfo.MemInfo,
			DiskInfo: nodeInfo.DiskInfo,
			HasSudo:  nodeInfo.HasSudo,
			Sysctl:   nodeInfo.Sysctl,
			Systemd:  nodeInfo.Systemd,
		}
		return describeDoneMsg{name: name, info: cached}
	}
}

func formatDescribeContent(name string, info *store.CachedNodeInfo) string {
	var b strings.Builder

	section := func(title string) {
		b.WriteString("\n")
		b.WriteString(lipgloss.NewStyle().Bold(true).Foreground(colorBlue).Render(title))
		b.WriteString("\n")
	}
	kv := func(key, val string) {
		k := lipgloss.NewStyle().Foreground(colorMauve).Width(16).Render(key)
		b.WriteString("  " + k + val + "\n")
	}

	b.WriteString(lipgloss.NewStyle().Bold(true).Foreground(colorTeal).Render(
		fmt.Sprintf(" System Info: %s", name)))
	b.WriteString("\n")
	if !info.FetchedAt.IsZero() {
		b.WriteString(lipgloss.NewStyle().Foreground(colorOverlay0).Render(
			fmt.Sprintf("  fetched %s", info.FetchedAt.Format("2006-01-02 15:04:05"))))
		b.WriteString("\n")
	}

	section("System")
	kv("Hostname", info.Hostname)
	kv("OS", info.OS)
	kv("Kernel", info.Kernel)

	section("CPU")
	kv("Model", info.CPUModel)
	kv("Cores", info.CPUCores)

	sudo := "No"
	if info.HasSudo {
		sudo = lipgloss.NewStyle().Foreground(colorGreen).Render("Yes")
	}
	kv("Sudo", sudo)

	section("Memory")
	for _, line := range strings.Split(info.MemInfo, "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			b.WriteString("  " + line + "\n")
		}
	}

	section("Disk")
	for _, line := range strings.Split(info.DiskInfo, "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			b.WriteString("  " + line + "\n")
		}
	}

	section("Sysctl")
	for _, line := range strings.Split(info.Sysctl, "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			b.WriteString("  " + line + "\n")
		}
	}

	section("Services (systemd)")
	for _, line := range strings.Split(info.Systemd, "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			b.WriteString("  " + line + "\n")
		}
	}

	return b.String()
}

func (a *App) openInNewTab(args ...string) tea.Cmd {
	return func() tea.Msg {
		spockBin, _ := os.Executable()
		if spockBin == "" {
			spockBin = os.Args[0]
		}
		err := openTerminalTab(spockBin, args)
		return tabOpenedMsg{err: err}
	}
}

// openTerminalTab opens a new Terminal.app window with the given command.
// Uses tmux if available, otherwise falls back to macOS Terminal.app / Linux terminals.
func openTerminalTab(bin string, args []string) error {
	fullCmd := bin + " " + strings.Join(args, " ")
	label := args[len(args)-1]

	// tmux — use if available
	if os.Getenv("TMUX") != "" {
		return exec.Command("tmux", "new-window", "-n", label, fullCmd).Run()
	}

	// macOS — always use Terminal.app
	if runtime.GOOS == "darwin" {
		script := fmt.Sprintf(`tell application "Terminal"
			activate
			do script "%s"
		end tell`, fullCmd)
		return exec.Command("osascript", "-e", script).Run()
	}

	// Linux fallback
	if runtime.GOOS == "linux" {
		for _, t := range []struct {
			bin  string
			args []string
		}{
			{"x-terminal-emulator", []string{"-e", "sh", "-c", fullCmd}},
			{"gnome-terminal", []string{"--", "sh", "-c", fullCmd}},
			{"konsole", []string{"-e", "sh", "-c", fullCmd}},
			{"xfce4-terminal", []string{"-e", fullCmd}},
			{"xterm", []string{"-e", "sh", "-c", fullCmd}},
		} {
			if _, err := exec.LookPath(t.bin); err == nil {
				if exec.Command(t.bin, t.args...).Start() == nil {
					return nil
				}
			}
		}
	}

	return fmt.Errorf("could not open terminal — run: %s", fullCmd)
}

func (a App) View() string {
	if a.quitting {
		return ""
	}
	if a.width == 0 {
		return "Loading..."
	}

	contentWidth := a.width - sidebarWidth - 3
	contentHeight := a.height - 4

	// Header
	header := a.renderHeader()

	// Sidebar
	sidebar := a.sidebar.View(a.focus == FocusSidebar, contentHeight)

	// Divider
	div := dividerStyle.Render(strings.Repeat("│\n", contentHeight))

	// Content
	var content string
	if a.form != nil {
		content = contentStyle.
			Width(contentWidth).
			Height(contentHeight).
			Render(a.form.View())
	} else {
		switch a.view {
		case ViewConnections:
			content = contentStyle.
				Width(contentWidth).
				Height(contentHeight).
				Render(connContentView(a.connTable, contentWidth, contentHeight, a.focus == FocusContent))
		case ViewSessions:
			content = contentStyle.
				Width(contentWidth).
				Height(contentHeight).
				Render(sessionContentView(a.sessionTable, contentWidth, contentHeight, a.focus == FocusContent))
		case ViewKeys:
			content = contentStyle.
				Width(contentWidth).
				Height(contentHeight).
				Render(keyContentView(a.keyTable, contentWidth, contentHeight, a.focus == FocusContent))
		}
	}

	// Main layout: sidebar | divider | content
	main := lipgloss.JoinHorizontal(lipgloss.Top, sidebar, div, content)

	// Status / help bar
	var footer string
	if a.statusMsg != "" {
		footer = a.statusStyle.Width(a.width).Render("  " + a.statusMsg)
	} else {
		footer = helpBar(a.view, a.focus, a.width)
	}

	// Overlays
	page := lipgloss.JoinVertical(lipgloss.Left, header, main, footer)

	if a.describeVisible {
		modalStyle := lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(colorMauve).
			Padding(0, 1).
			Width(a.width - 8).
			Height(a.height - 6)
		helpLine := lipgloss.NewStyle().Foreground(colorOverlay0).Render(
			"  ↑↓ scroll · R refresh · esc close")
		modalContent := a.describeVP.View() + "\n" + helpLine
		overlay := modalStyle.Render(modalContent)
		return placeOverlay(a.width, a.height, overlay, page)
	}

	if a.dialog != nil {
		overlay := a.dialog.View(a.width)
		return placeOverlay(a.width, a.height, overlay, page)
	}

	return page
}

func (a App) renderHeader() string {
	title := headerTitleStyle.Render(" SPOCK")
	viewName := headerViewStyle.Render("▸ " + viewLabels[a.view])

	left := title
	right := viewName

	gap := a.width - lipgloss.Width(left) - lipgloss.Width(right) - 2
	if gap < 1 {
		gap = 1
	}

	return headerStyle.Width(a.width).Render(
		left + strings.Repeat(" ", gap) + right,
	)
}

// placeOverlay centers an overlay on top of background content.
func placeOverlay(width, height int, overlay, background string) string {
	bgLines := strings.Split(background, "\n")
	olLines := strings.Split(overlay, "\n")

	olW := lipgloss.Width(overlay)
	startRow := (height - len(olLines)) / 2
	startCol := (width - olW) / 2
	if startRow < 0 {
		startRow = 0
	}
	if startCol < 0 {
		startCol = 0
	}

	for i, olLine := range olLines {
		row := startRow + i
		if row >= len(bgLines) {
			break
		}
		bgLine := bgLines[row]
		// Pad background line to width
		for lipgloss.Width(bgLine) < startCol {
			bgLine += " "
		}
		// Replace portion with overlay
		before := truncateToWidth(bgLine, startCol)
		bgLines[row] = before + olLine
	}

	return strings.Join(bgLines, "\n")
}

func truncateToWidth(s string, w int) string {
	// Simple rune-based truncation
	runes := []rune(s)
	if len(runes) > w {
		return string(runes[:w])
	}
	result := string(runes)
	for len(result) < w {
		result += " "
	}
	return result
}

func copyToClipboard(text string) bool {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("pbcopy")
	case "linux":
		if _, err := exec.LookPath("xclip"); err == nil {
			cmd = exec.Command("xclip", "-selection", "clipboard")
		} else if _, err := exec.LookPath("xsel"); err == nil {
			cmd = exec.Command("xsel", "--clipboard", "--input")
		}
	}
	if cmd == nil {
		return false
	}
	cmd.Stdin = strings.NewReader(text)
	return cmd.Run() == nil
}

// Run starts the TUI application.
func Run() error {
	app := NewApp()
	p := tea.NewProgram(app, tea.WithAltScreen())
	_, err := p.Run()
	return err
}
