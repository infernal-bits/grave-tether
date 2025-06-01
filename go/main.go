package main

import (
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// --- Configuration ---
const (
	adbPath      = "adb" // Assumes 'adb' is in your PATH. Specify full path if not, e.g., "/usr/bin/adb"
	deviceSerial = ""    // Leave empty to auto-select. Set to your phone's serial if multiple devices.
)

// --- Styling with Lipgloss ---
var (
	headerStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#9B59B6")). // Purple
			Align(lipgloss.Center).
			MarginBottom(1)

	statusBoxStyle = lipgloss.NewStyle().
			Border(lipgloss.ThickBorder()).
			BorderForeground(lipgloss.Color("#3498DB")). // Blue
			Padding(1, 2).
			Margin(1, 2).
			Height(7) // Adjusted to fit 3 lines of status + padding

	statusTitleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#6A5ACD")). // Slate Blue
			Align(lipgloss.Center)

	messageLogStyle = lipgloss.NewStyle().
			Border(lipgloss.ThickBorder()).
			BorderForeground(lipgloss.Color("#5DADE2")). // Light Blue
			Padding(0, 1).
			Margin(1, 2).
			Height(5) // Adjusted to fit 2 lines of log + padding

	messageLogTitleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#6A5ACD")). // Slate Blue
			Align(lipgloss.Center)

	menuStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#8E44AD")). // Purple
			Padding(1, 2).
			Margin(1, 2).
			Height(5) // Adjusted for menu items

	menuTitleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#8E44AD")). // Purple
			Align(lipgloss.Center)

	statusOKStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("#2ECC71")).Bold(true) // Green
	statusWarnStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#F1C40F")).Bold(true) // Yellow
	statusErrStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("#E74C3C")).Bold(true) // Red
	statusInfoStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#FFFFFF"))          // White

	logMsgStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("#A0A0A0"))          // Dim white for log
	logSuccessStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#2ECC71")).Bold(true) // Green
	logErrorStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("#E74C3C")).Bold(true) // Red
	logInfoStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("#3498DB")).Bold(true) // Blue
	logWarnStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("#F1C40F")).Bold(true) // Yellow
)

// --- Model Definition (The Elm Architecture) ---

type model struct {
	adbStatus        string
	deviceInfo       string
	linuxIfaceStatus string
	adbReady         bool
	currentSerial    string // Actual serial used for commands (from config or auto-detected)

	log       []string
	logScroll int // Not used in this simple log display, but good for future scrolling

	choices []string
	cursor  int // for menu selection
}

// Msg types for async operations (results returned by commands)
type adbStatusMsg struct {
	status string
	info   string
	ready  bool
	serial string // The serial detected/used
	logMsg string
}

type linuxIfaceStatusMsg struct {
	status string
	logMsg string
}

type adbCommandResultMsg struct {
	description string
	success     bool
	output      string // Combined stdout/stderr
}

// --- Initial Model State ---
func initialModel() model {
	return model{
		adbStatus:        statusInfoStyle.Render("Unknown"),
		deviceInfo:       statusInfoStyle.Render("N/A"),
		linuxIfaceStatus: statusInfoStyle.Render("Unknown"),
		adbReady:         false,
		currentSerial:    deviceSerial, // Initialize with config serial, or empty
		log:              []string{},
		logScroll:        0,
		choices:          []string{"Enable Tethering", "Disable Tethering", "Refresh Status", "Exit"},
		cursor:           0,
	}
}

// --- Init (Initial command to run when the program starts) ---
func (m model) Init() tea.Cmd {
	return tea.Batch(checkAdbConnectionCmd(m.currentSerial), getLinuxInterfaceStatusCmd())
}

// --- Update (Handle messages and update model state) ---
func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit // Quit the application
		case "up", "k":
			if m.cursor > 0 {
				m.cursor-- // Move cursor up
			}
		case "down", "j":
			if m.cursor < len(m.choices)-1 {
				m.cursor++ // Move cursor down
			}
		case "enter":
			switch m.cursor {
			case 0: // Enable Tethering
				if !m.adbReady {
					m.addLog(logErrorStyle.Render("ADB not connected or authorized. Cannot enable tethering."))
					return m, nil
				}
				m.addLog(logInfoStyle.Render("Attempting to ENABLE tethering..."))
				return m, tea.Sequence(
					// Pass m.currentSerial to the command functions
					runAdbCommandCmd(m.currentSerial, "Attempting ADB root", "root"), // Often fails on stock ROMs, but harmless
					runAdbCommandCmd(m.currentSerial, "Set tether_dun_required to 0", "shell settings put global tether_dun_required 0"),
					runAdbCommandCmd(m.currentSerial, "Set USB functions to RNDIS", "shell svc usb setFunctions rndis"),
					runAdbCommandCmd(m.currentSerial, "Disable tether_offload", "shell settings put global tether_offload_disabled 1"),
					func() tea.Msg { return adbCommandResultMsg{description: "Tethering commands sent. Refreshing status.", success: true} }, // Synthetic success message
					checkAdbConnectionCmd(m.currentSerial), // Re-check status
					getLinuxInterfaceStatusCmd(),            // Re-check status
				)
			case 1: // Disable Tethering
				if !m.adbReady {
					m.addLog(logErrorStyle.Render("ADB not connected or authorized. Cannot disable tethering."))
					return m, nil
				}
				m.addLog(logInfoStyle.Render("Attempting to DISABLE tethering..."))
				return m, tea.Sequence(
					// Pass m.currentSerial to the command functions
					runAdbCommandCmd(m.currentSerial, "Restore tether_offload_disabled to 0", "shell settings put global tether_offload_disabled 0"),
					runAdbCommandCmd(m.currentSerial, "Restore tether_dun_required to 1", "shell settings put global tether_dun_required 1"),
					runAdbCommandCmd(m.currentSerial, "Restore default USB functions (MTP, ADB)", "shell svc usb setFunctions mtp,adb"),
					func() tea.Msg { return adbCommandResultMsg{description: "Tethering deactivation commands sent. Refreshing status.", success: true} }, // Synthetic success message
					checkAdbConnectionCmd(m.currentSerial), // Re-check status
					getLinuxInterfaceStatusCmd(),            // Re-check status
				)
			case 2: // Refresh Status
				m.addLog(logInfoStyle.Render("Refreshing status..."))
				return m, tea.Batch(checkAdbConnectionCmd(m.currentSerial), getLinuxInterfaceStatusCmd())
			case 3: // Exit
				return m, tea.Quit
			}

		case "r": // Binding for Refresh Status
			m.addLog(logInfoStyle.Render("Refreshing status..."))
			return m, tea.Batch(checkAdbConnectionCmd(m.currentSerial), getLinuxInterfaceStatusCmd())
		}

	// Handle messages returning from async commands
	case adbStatusMsg:
		m.adbStatus = msg.status
		m.deviceInfo = msg.info
		m.adbReady = msg.ready
		m.currentSerial = msg.serial // Update the model's currentSerial
		if msg.logMsg != "" {
			m.addLog(msg.logMsg)
		}
		return m, nil

	case linuxIfaceStatusMsg:
		m.linuxIfaceStatus = msg.status
		if msg.logMsg != "" {
			m.addLog(msg.logMsg)
		}
		return m, nil

	case adbCommandResultMsg:
		var logLine string
		if msg.success {
			logLine = logSuccessStyle.Render(fmt.Sprintf("SUCCESS: %s", msg.description))
		} else {
			logLine = logErrorStyle.Render(fmt.Sprintf("FAILED: %s", msg.description))
			if msg.output != "" {
				// Truncate long output lines for log readability
				outputStr := strings.TrimSpace(msg.output)
				if len(outputStr) > 80 { // Arbitrary length for log
					outputStr = outputStr[:77] + "..."
				}
				logLine += logErrorStyle.Render(fmt.Sprintf(" (Output: %s)", outputStr))
			}
		}
		m.addLog(logLine)
		return m, nil
	}

	return m, cmd
}

// --- View (Render the UI based on the current model state) ---
func (m model) View() string {
	s := strings.Builder{}

	// Header
	headerText := fmt.Sprintf(
		"┌───────────────────────────────────────────┐\n" +
			"│       %sADB Tethering for Arch Linux%s        │\n" +
			"│          %sSamsung Galaxy S23 Ultra%s           │\n" +
			"└───────────────────────────────────────────┘",
		lipgloss.NewStyle().Foreground(lipgloss.Color("#F1C40F")).Render, // Yellow for ADB Tethering
		lipgloss.NewStyle().Foreground(lipgloss.Color("#9B59B6")).Render, // Purple for border
		lipgloss.NewStyle().Foreground(lipgloss.Color("#2ECC71")).Render, // Green for Samsung Galaxy
		lipgloss.NewStyle().Foreground(lipgloss.Color("#9B59B6")).Render, // Purple for border
	)
	s.WriteString(headerStyle.Render(headerText) + "\n")

	// Status Box
	statusContent := fmt.Sprintf(
		"ADB Connection:  %s\n"+
			"Detected Device: %s\n"+
			"Linux Interface: %s",
		m.adbStatus, m.deviceInfo, m.linuxIfaceStatus,
	)
	s.WriteString(statusBoxStyle.Render(
		statusTitleStyle.Render("SYSTEM STATUS") + "\n" +
			statusContent,
	) + "\n")

	// Message Log
	logContent := ""
	// Display last 2 lines of log
	start := 0
	if len(m.log) > 2 {
		start = len(m.log) - 2
	}
	for i := start; i < len(m.log); i++ {
		logContent += m.log[i] + "\n"
	}

	s.WriteString(messageLogStyle.Render(
		messageLogTitleStyle.Render("MESSAGES / OUTPUT") + "\n" +
			logContent,
	) + "\n")

	// Menu
	menuContent := ""
	for i, choice := range m.choices {
		cursor := "  " // default cursor
		choiceStyled := lipgloss.NewStyle().Foreground(lipgloss.Color("#FFFFFF")).Render(choice) // Default white text
		if m.cursor == i {
			cursor = lipgloss.NewStyle().Foreground(lipgloss.Color("#2ECC71")).Render("› ") // green arrow
			choiceStyled = lipgloss.NewStyle().Foreground(lipgloss.Color("#2ECC71")).Bold(true).Render(choice) // Green bold when selected
		}
		menuContent += fmt.Sprintf("%s%s\n", cursor, choiceStyled)
	}

	s.WriteString(menuStyle.Render(
		menuTitleStyle.Render("MENU") + "\n" +
			menuContent,
	) + "\n")

	// Footer with keybindings
	s.WriteString(lipgloss.NewStyle().Align(lipgloss.Center).Render(
		lipgloss.NewStyle().Foreground(lipgloss.Color("#95A5A6")).Render("Use ↑↓ to navigate, Enter to select, Q/Ctrl+C to quit, R to refresh"),
	) + "\n")

	return s.String()
}

// --- Helper for logging within the model ---
func (m *model) addLog(line string) {
	m.log = append(m.log, line)
	// Keep log short to avoid overflowing the box (e.g., last 10 lines)
	if len(m.log) > 10 {
		m.log = m.log[len(m.log)-10:]
	}
}

// --- Commands (Async operations executed by the Bubble Tea runtime) ---

func checkAdbConnectionCmd(currentSerial string) tea.Cmd {
	return func() tea.Msg {
		cmd := exec.Command(adbPath, "devices")
		output, err := cmd.Output()
		if err != nil {
			return adbStatusMsg{
				status: statusErrStyle.Render("ERROR: ADB not found"),
				info:   statusInfoStyle.Render("N/A"),
				ready:  false,
				logMsg: logErrorStyle.Render(fmt.Sprintf("ADB command not found or error: %v", err)),
			}
		}

		lines := strings.Split(strings.TrimSpace(string(output)), "\n")
		var devices []string
		for _, line := range lines {
			if strings.Contains(line, "device") && !strings.Contains(line, "List of devices attached") {
				parts := strings.Split(line, "\t")
				if len(parts) > 0 {
					devices = append(devices, parts[0])
				}
			}
		}

		selectedSerial := currentSerial // Start with the serial passed from the model
		if deviceSerial != "" { // If DEVICE_SERIAL const is set, it overrides
			selectedSerial = deviceSerial
		}

		if len(devices) == 0 {
			return adbStatusMsg{
				status: statusErrStyle.Render("Disconnected"),
				info:   statusInfoStyle.Render("N/A"),
				ready:  false,
				logMsg: logErrorStyle.Render("No Android device detected. Connect phone & enable USB debugging."),
			}
		} else if len(devices) > 1 && selectedSerial == "" {
			// Multiple devices found, and no specific serial configured or detected yet
			return adbStatusMsg{
				status: statusWarnStyle.Render("Multiple Devices"),
				info:   statusInfoStyle.Render(fmt.Sprintf("Specify serial (found: %s)", strings.Join(devices, ", "))),
				ready:  false,
				logMsg: logWarnStyle.Render(fmt.Sprintf("Multiple devices found. Please set `deviceSerial` in the script, or specify the serial number: %s", strings.Join(devices, ", "))),
			}
		} else {
			// If selectedSerial is still empty, and only one device, auto-select it
			if selectedSerial == "" && len(devices) == 1 {
				selectedSerial = devices[0]
			} else { // Verify specified serial is present
				found := false
				for _, d := range devices {
					if d == selectedSerial {
						found = true
						break
					}
				}
				if !found {
					return adbStatusMsg{
						status: statusErrStyle.Render("Specified Device Not Found"),
						info:   statusInfoStyle.Render(selectedSerial),
						ready:  false,
						logMsg: logErrorStyle.Render(fmt.Sprintf("Specified device (%s) not found among connected devices.", selectedSerial)),
					}
				}
			}

			// Test authorization
			testCmd := exec.Command(adbPath, "-s", selectedSerial, "shell", "echo", "test")
			testOutput, testErr := testCmd.CombinedOutput()
			if testErr != nil || strings.Contains(strings.ToLower(string(testOutput)), "unauthorized") {
				return adbStatusMsg{
					status: statusErrStyle.Render("Auth Failed"),
					info:   statusInfoStyle.Render(selectedSerial),
					ready:  false,
					serial: selectedSerial, // Still return serial even if auth failed
					logMsg: logErrorStyle.Render(fmt.Sprintf("Failed to communicate with %s. Ensure authorization (prompt on phone).", selectedSerial)),
				}
			}

			return adbStatusMsg{
				status: statusOKStyle.Render("Connected"),
				info:   statusInfoStyle.Render(selectedSerial),
				ready:  true,
				serial: selectedSerial,
				logMsg: logSuccessStyle.Render(fmt.Sprintf("ADB connection established with %s.", selectedSerial)),
			}
		}
	}
}

func getLinuxInterfaceStatusCmd() tea.Cmd {
	return func() tea.Msg {
		cmd := exec.Command("ip", "link", "show")
		output, err := cmd.Output()
		if err != nil {
			return linuxIfaceStatusMsg{
				status: statusErrStyle.Render("ERROR: 'ip' missing"),
				logMsg: logErrorStyle.Render(fmt.Sprintf("'ip' command not found or error: %v", err)),
			}
		}

		// Regex to find common RNDIS/USB tethering interface names with UP status
		re := regexp.MustCompile(`\d+: (usb\d+|rndis\d+|enp\S+u\d+): <.*BROADCAST,MULTICAST,UP.*>`)
		match := re.FindStringSubmatch(string(output))

		if len(match) > 1 {
			interfaceName := match[1]
			ipCmd := exec.Command("ip", "-4", "addr", "show", "dev", interfaceName)
			ipOutput, ipErr := ipCmd.Output()

			if ipErr == nil {
				ipRe := regexp.MustCompile(`inet (\d{1,3}\.\d{1,3}\.\d{1,3}\.\d{1,3})`)
				ipMatch := ipRe.FindStringSubmatch(string(ipOutput))
				if len(ipMatch) > 1 {
					return linuxIfaceStatusMsg{
						status: statusOKStyle.Render(fmt.Sprintf("%s (IP: %s)", interfaceName, ipMatch[1])),
						logMsg: logSuccessStyle.Render(fmt.Sprintf("Linux interface '%s' detected with IP: %s.", interfaceName, ipMatch[1])),
					}
				}
			}
			return linuxIfaceStatusMsg{
				status: statusWarnStyle.Render(fmt.Sprintf("%s (No IP)", interfaceName)),
				logMsg: logWarnStyle.Render(fmt.Sprintf("Linux interface '%s' detected, but no IP. Needs DHCP config.", interfaceName)),
			}
		}
		return linuxIfaceStatusMsg{
			status: statusErrStyle.Render("Not Detected"),
			logMsg: logErrorStyle.Render("No active USB-related network interface found on Linux."),
		}
	}
}

func runAdbCommandCmd(serial, description string, shellCommand string) tea.Cmd {
	return func() tea.Msg {
		if serial == "" {
			return adbCommandResultMsg{description: description, success: false, output: "Error: Device serial is not available."}
		}

		// shellCommand can be "root" or "shell settings put global ..."
		var cmd *exec.Cmd
		if strings.HasPrefix(shellCommand, "shell ") {
			// For "shell" commands, split "shell" from the rest
			parts := strings.SplitN(shellCommand, " ", 2)
			if len(parts) < 2 { // Should not happen if shellCommand is properly formed
				return adbCommandResultMsg{description: description, success: false, output: fmt.Sprintf("Invalid shell command format: %s", shellCommand)}
			}
			cmd = exec.Command(adbPath, "-s", serial, parts[0], parts[1])
		} else {
			// For non-shell commands like "root"
			cmd = exec.Command(adbPath, "-s", serial, shellCommand)
		}

		output, err := cmd.CombinedOutput() // Capture both stdout and stderr
		success := err == nil

		return adbCommandResultMsg{description: description, success: success, output: string(output)}
	}
}

func main() {
	p := tea.NewProgram(initialModel(), tea.WithAltScreen()) // WithAltScreen makes it full-screen
	if _, err := p.Run(); err != nil {
		fmt.Printf("Alas, there's been an error: %v\n", err)
		os.Exit(1)
	}
}
