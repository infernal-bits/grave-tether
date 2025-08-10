// main.go
//
// This application provides a professional Terminal User Interface (TUI) to manage
// USB tethering for an Android device from a Linux machine. It uses adb
// to communicate with the device and provides real-time status updates in a
// clean, multi-pane layout.
//
// Dependencies:
// - Go (https://golang.org/)
// - adb (Android Debug Bridge) must be installed and in the system's PATH.
// - Bubble Tea & Lipgloss libraries (will be fetched by `go mod tidy`)
//
// To run this application:
// 1. Save the code as `main.go`.
// 2. Open a terminal in the same directory.
// 3. Run `go mod init go-tether-pro` (only the first time).
// 4. Run `go mod tidy` to download dependencies.
// 5. Run `go run main.go`.

package main

import (
	"fmt"
	"log"
	"net"
	"os"
	"os/exec"
	"strings"
	"time"
	
    "github.com/charmbracelet/bubbles/spinner"
    github.com/fatih/color "github.com/fatih/color"
    "github.com/charmbracelet/bubbles/viewport"
    tea "github.com/charmbracelet/bubbletea"
    "github.com/charmbracelet/lipgloss"
    "github.com/mbndr/figlet4go"
)

// --- STYLES ---

const (
	colorPurple    = lipgloss.Color("57")
	colorGray      = lipgloss.Color("240")
	colorGreen     = lipgloss.Color("78")
	colorRed       = lipgloss.Color("197")
	colorYellow    = lipgloss.Color("228")
	colorBlue      = lipgloss.Color("63")
	colorDarkGray  = lipgloss.Color("235")
	colorLightGray = lipgloss.Color("244")
)

var (
	// General layout styling
	appStyle = lipgloss.NewStyle().Margin(1, 2)

	// Banner Style
	bannerStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FAFAFA")).
			Background(colorPurple).
			Padding(0, 1)

	// Box styles
	boxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(colorBlue).
			Padding(0, 1)

	boxHeaderStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(colorPurple).
			Padding(0, 1)

	// Menu styles
	menuStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(colorBlue).
			Padding(1, 2)
	menuCursorStyle = lipgloss.NewStyle().Foreground(colorPurple).Bold(true)
	menuItemStyle   = lipgloss.NewStyle().Foreground(colorLightGray)

	// Status styles
	statusKeyStyle = lipgloss.NewStyle().Foreground(colorLightGray)
	statusOKStyle  = lipgloss.NewStyle().Foreground(colorGreen)
	statusErrStyle = lipgloss.NewStyle().Foreground(colorRed)
	statusMidStyle = lipgloss.NewStyle().Foreground(colorYellow)
	statusUnset    = lipgloss.NewStyle().Foreground(colorGray)
)

// --- LOGGING & STATE ---

type logEntry struct {
	timestamp time.Time
	level     string
	message   string
}

const (
	logLevelInfo  = "INFO"
	logLevelWarn  = "WARN"
	logLevelError = "ERROR"
	logLevelCmd   = "CMD"
)

// --- ADB COMMANDS ---

const (
	defaultUSBFunction = "mtp"
	tetherUSBFunction  = "rndis"
)

// --- MODEL ---

type model struct {
	spinner     spinner.Model
	logView     viewport.Model
	width       int
	height      int
	ready       bool
	isLoading   bool
	lastLog     string
	logs        []logEntry
	status      appStatus
	error       error
	menuChoices []string
	menuCursor  int
}

type appStatus struct {
	adbPath               string
	deviceConnected       bool
	deviceName            string
	isTethering           bool
	currentUSBFunction    string
	tetherDunRequired     string
	tetherOffloadDisabled string
	networkInterfaceName  string
	networkIP             string
}

// --- MESSAGES ---

type statusUpdateMsg struct {
	status appStatus
	err    error
}
type logMsg logEntry
type tetheringDoneMsg struct{ err error }

// --- HELPER FUNCTIONS ---

func runCmd(name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return string(output), fmt.Errorf("failed to run '%s %s': %w. Output: %s", name, strings.Join(args, " "), err, string(output))
	}
	return strings.TrimSpace(string(output)), nil
}

// --- CORE LOGIC ---

func addLog(level, message string) tea.Cmd {
	return func() tea.Msg {
		return logMsg{timestamp: time.Now(), level: level, message: message}
	}
}

func checkStatus() tea.Cmd {
	return func() tea.Msg {
		var status appStatus
		var err error

		status.adbPath, err = exec.LookPath("adb")
		if err != nil {
			return statusUpdateMsg{err: fmt.Errorf("adb not found in PATH")}
		}

		out, err := runCmd(status.adbPath, "devices")
		if err != nil {
			return statusUpdateMsg{err: err}
		}
		lines := strings.Split(out, "\n")
		if len(lines) > 1 && strings.Contains(lines[1], "device") {
			status.deviceConnected = true
			model, _ := runCmd(status.adbPath, "shell", "getprop", "ro.product.model")
			status.deviceName = model
		} else {
			status.deviceConnected = false
			status.deviceName = "No device found"
			return statusUpdateMsg{status: status}
		}

		status.currentUSBFunction, _ = runCmd(status.adbPath, "shell", "svc", "usb", "getFunctions")
		status.isTethering = status.currentUSBFunction == tetherUSBFunction
		status.tetherDunRequired, _ = runCmd(status.adbPath, "shell", "settings", "get", "global", "tether_dun_required")
		status.tetherOffloadDisabled, _ = runCmd(status.adbPath, "shell", "settings", "get", "global", "tether_offload_disabled")

		status.networkInterfaceName = "Not found"
		status.networkIP = "N/A"
		if status.isTethering {
			interfaces, err := net.Interfaces()
			if err == nil {
				for _, i := range interfaces {
					if strings.HasPrefix(i.Name, "usb") || strings.HasPrefix(i.Name, "rndis") {
						addrs, err := i.Addrs()
						if err == nil {
							for _, addr := range addrs {
								var ip net.IP
								switch v := addr.(type) {
								case *net.IPNet:
									ip = v.IP
								case *net.IPAddr:
									ip = v.IP
								}
								if ip != nil && ip.To4() != nil {
									status.networkInterfaceName = i.Name
									status.networkIP = ip.String()
									break
								}
							}
						}
					}
					if status.networkIP != "N/A" {
						break
					}
				}
			}
		}
		return statusUpdateMsg{status: status}
	}
}

func toggleTethering(enable bool) tea.Cmd {
	return func() tea.Msg {
		adbPath, err := exec.LookPath("adb")
		if err != nil {
			return tetheringDoneMsg{err: err}
		}

		if enable {
			log.Println("Enabling tethering...")
			_, err = runCmd(adbPath, "shell", "settings", "put", "global", "tether_dun_required", "0")
			if err != nil {
				return tetheringDoneMsg{err: err}
			}
			_, err = runCmd(adbPath, "shell", "svc", "usb", "setFunctions", tetherUSBFunction)
			if err != nil {
				return tetheringDoneMsg{err: err}
			}
			time.Sleep(2 * time.Second)
			_, err = runCmd(adbPath, "shell", "settings", "put", "global", "tether_offload_disabled", "1")
			if err != nil {
				return tetheringDoneMsg{err: err}
			}
		} else {
			log.Println("Disabling tethering...")
			_, err = runCmd(adbPath, "shell", "svc", "usb", "setFunctions", defaultUSBFunction)
			if err != nil {
				return tetheringDoneMsg{err: err}
			}
		}

		log.Println("Tethering action complete.")
		return tetheringDoneMsg{err: nil}
	}
}

// --- BUBBLE TEA IMPLEMENTATION ---

func initialModel() model {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(colorPurple)

	return model{
		spinner:   s,
		isLoading: true,
		logs: []logEntry{
			{timestamp: time.Now(), level: logLevelInfo, message: "Go Tether TUI Initializing..."},
		},
		menuChoices: []string{
			"Enable Tethering",
			"Disable Tethering",
			"Refresh & Flush Log",
			"Daemonize (placeholder)",
			"Exit",
		},
		menuCursor: 0,
	}
}

func (m model) Init() tea.Cmd {
	return tea.Batch(
		m.spinner.Tick,
		addLog(logLevelInfo, "Checking for ADB..."),
		checkStatus(),
	)
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		leftPaneWidth := m.width/3 + 2
		logViewWidth := m.width - leftPaneWidth - appStyle.GetHorizontalFrameSize() - 2
		logViewHeight := m.height - appStyle.GetVerticalFrameSize() - 5
		m.logView = viewport.New(logViewWidth, logViewHeight)
		m.logView.Style = boxStyle
		m.ready = true

	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "up", "k":
			if m.menuCursor > 0 {
				m.menuCursor--
			}
		case "down", "j":
			if m.menuCursor < len(m.menuChoices)-1 {
				m.menuCursor++
			}
		case "enter":
			switch m.menuCursor {
			case 0: // Enable Tethering
				if m.status.deviceConnected {
					m.isLoading = true
					m.lastLog = "Enabling tethering..."
					cmds = append(cmds, addLog(logLevelCmd, m.lastLog), toggleTethering(true))
				} else {
					cmds = append(cmds, addLog(logLevelWarn, "Cannot enable: No device connected."))
				}
			case 1: // Disable Tethering
				if m.status.deviceConnected {
					m.isLoading = true
					m.lastLog = "Disabling tethering..."
					cmds = append(cmds, addLog(logLevelCmd, m.lastLog), toggleTethering(false))
				} else {
					cmds = append(cmds, addLog(logLevelWarn, "Cannot disable: No device connected."))
				}
			case 2: // Refresh & Flush
				m.isLoading = true
				m.lastLog = "Refreshing status..."
				m.logs = []logEntry{} // Flush logs
				cmds = append(cmds, addLog(logLevelCmd, "Log flushed and status refreshed."), checkStatus())
			case 3: // Daemonize
				// NOTE: True daemonization is complex. This is a placeholder.
				cmds = append(cmds, addLog(logLevelWarn, "Daemonize feature is not yet implemented."))
			case 4: // Exit
				return m, tea.Quit
			}
		}

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		cmds = append(cmds, cmd)

	case logMsg:
		m.logs = append(m.logs, logEntry(msg))

	case statusUpdateMsg:
		m.isLoading = false
		m.lastLog = ""
		if msg.err != nil {
			m.error = msg.err
			cmds = append(cmds, addLog(logLevelError, msg.err.Error()))
		} else {
			m.status = msg.status
			cmds = append(cmds, addLog(logLevelInfo, "Status updated successfully."))
		}

	case tetheringDoneMsg:
		m.isLoading = false
		if msg.err != nil {
			m.error = msg.err
			cmds = append(cmds, addLog(logLevelError, "Tethering command failed: "+msg.err.Error()))
		} else {
			m.error = nil
			cmds = append(cmds, addLog(logLevelInfo, "Tethering action successful. Refreshing..."))
		}
		cmds = append(cmds, checkStatus())
	}

	if m.ready {
		var logLines []string
		for _, l := range m.logs {
			levelColor := colorGray
			switch l.level {
			case logLevelInfo:
				levelColor = colorGreen
			case logLevelWarn:
				levelColor = colorYellow
			case logLevelError:
				levelColor = colorRed
			case logLevelCmd:
				levelColor = colorPurple
			}
			ts := l.timestamp.Format("15:04:05")
			logLines = append(logLines, fmt.Sprintf("%s [%s] %s", ts, lipgloss.NewStyle().Foreground(levelColor).Render(l.level), l.message))
		}
		m.logView.SetContent(strings.Join(logLines, "\n"))
		m.logView.GotoBottom()
	}

	var cmd tea.Cmd
	m.logView, cmd = m.logView.Update(msg)
	cmds = append(cmds, cmd)

	return m, tea.Batch(cmds...)
}

func (m model) View() string {
	if !m.ready {
		return "Initializing..."
	}

	// --- RENDER PANES ---
	leftColumnWidth := m.width / 3
	devicePane := m.renderDevicePane(leftColumnWidth)
	menuPane := m.renderMenuPane(leftColumnWidth)

	leftColumn := lipgloss.JoinVertical(lipgloss.Left,
		devicePane,
		menuPane,
	)

	rightColumn := lipgloss.JoinVertical(lipgloss.Left,
		boxHeaderStyle.Render("EVENT LOG"),
		m.logView.View(),
	)

	// --- RENDER FOOTER ---
	loading := ""
	if m.isLoading {
		loading = fmt.Sprintf("%s %s", m.spinner.View(), m.lastLog)
	}

	mainContent := lipgloss.JoinVertical(lipgloss.Left,
		m.renderBanner(),
		lipgloss.JoinHorizontal(lipgloss.Top, leftColumn, rightColumn),
		loading,
	)

	return appStyle.Render(mainContent)
}

func (m model) renderBanner() string {
    // Generate figlet text
    figletText := generateFigletText()

    // Create a style for the figlet text
    figletStyle := lipgloss.NewStyle().
        Bold(true).
        Foreground(lipgloss.Color("#FAFAFA")).
        Background(colorPurple).
        Padding(0, 1)

    // Join the figlet text with spaces to fit the width
    joinedFigletText := strings.Join(figletText, " ")

    return figletStyle.Render(joinedFigletText)
}

// New function to generate figlet text
func generateFigletText() []string {
    // Replace this with actual figlet generation logic
    return []string{"Tar'd 'N Tethered"}
}
func (m model) renderDevicePane(width int) string {
	statusText := statusErrStyle.Render("Disconnected")
	if m.status.deviceConnected {
		statusText = statusOKStyle.Render("Connected")
	}
	stateText := statusMidStyle.Render("Disabled")
	if m.status.isTethering {
		stateText = statusOKStyle.Render("Enabled")
	}
	ipText := statusMidStyle.Render(m.status.networkInterfaceName)
	if m.status.networkIP != "N/A" {
		ipText = statusOKStyle.Render(m.status.networkIP)
	}

	content := lipgloss.JoinVertical(lipgloss.Left,
		fmt.Sprintf("%s %s", statusKeyStyle.Render("Status:"), statusText),
		fmt.Sprintf("%s %s", statusKeyStyle.Render("Device:"), m.status.deviceName),
		fmt.Sprintf("%s %s", statusKeyStyle.Render("State:"), stateText),
		fmt.Sprintf("%s %s", statusKeyStyle.Render("USB Mode:"), m.status.currentUSBFunction),
		fmt.Sprintf("%s %s", statusKeyStyle.Render("Interface:"), m.status.networkInterfaceName),
		fmt.Sprintf("%s %s", statusKeyStyle.Render("IP Address:"), ipText),
	)
	
	return boxStyle.Width(width).Render(
		lipgloss.JoinVertical(lipgloss.Left, 
			boxHeaderStyle.Render("CURRENT STATUS:"), 
			content,
		),
	)
}


func (m model) renderMenuPane(width int) string {
	var b strings.Builder
	for i, choice := range m.menuChoices {
		var item string
		if m.menuCursor == i {
			item = menuCursorStyle.Render("> " + choice)
		} else {
			item = menuItemStyle.Render("  " + choice)
		}
		b.WriteString(item + "\n")
	}

	return menuStyle.Width(width).Render(
		lipgloss.JoinVertical(lipgloss.Left, boxHeaderStyle.Render("COMMANDS"), b.String()),
	)
}

// --- MAIN FUNCTION ---

func main() {
	f, err := os.OpenFile("go-tether.log", os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
	if err != nil {
		log.Fatalf("error opening file: %v", err)
	}
	defer f.Close()
	log.SetOutput(f)
	log.Println("Application starting...")

	p := tea.NewProgram(initialModel(), tea.WithAltScreen(), tea.WithMouseCellMotion())
	if _, err := p.Run(); err != nil {
		fmt.Printf("Alas, there's been an error: %v", err)
		os.Exit(1)
	}
}
