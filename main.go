package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	textinput "github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	resty "github.com/go-resty/resty/v2"
)

var version = "unknown"

var (
	colorRed       = lipgloss.Color("131") // muted red
	colorYellow    = lipgloss.Color("143") // muted yellow
	colorLightGray = lipgloss.Color("250") // lighter gray for keys
	colorGray      = lipgloss.Color("245") // gray for descriptions

	logoStyle  = lipgloss.NewStyle().Foreground(colorRed).Bold(true).Padding(1, 1)
	headerBar  = lipgloss.NewStyle().Bold(true).Padding(1, 1)
	inputBox   = lipgloss.NewStyle().Border(lipgloss.DoubleBorder(), false, false, false, true).BorderForeground(colorYellow)
	inputLabel = lipgloss.NewStyle().Foreground(colorYellow).Bold(true)
	timerBox   = lipgloss.NewStyle().Foreground(colorYellow).Bold(true).Padding(0, 2).MarginRight(2)
	pausedBox  = lipgloss.NewStyle().Foreground(colorRed).Bold(true).Underline(true).Padding(0, 2)
	msgStyle   = lipgloss.NewStyle().Foreground(colorRed).Italic(true)
)

type timerMsg time.Duration

type screen int

const (
	screenMainApp screen = iota
	screenConfirmCancel
	screenRecoverTimer
)

type keyMap struct {
	Quit   key.Binding
	Start  key.Binding
	Submit key.Binding
	Pause  key.Binding
	Resume key.Binding
	Cancel key.Binding
	Up     key.Binding
	Down   key.Binding
	Help   key.Binding
}

func (k keyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Start, k.Submit, k.Help, k.Quit}
}

func (k keyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.Start, k.Submit, k.Pause, k.Resume},
		{k.Cancel, k.Up, k.Down, k.Help, k.Quit},
	}
}

var keys = keyMap{
	Quit: key.NewBinding(
		key.WithKeys("q", "ctrl+c"),
		key.WithHelp("q", "quit"),
	),
	Start: key.NewBinding(
		key.WithKeys("enter"),
		key.WithHelp("enter", "start timer"),
	),
	Submit: key.NewBinding(
		key.WithKeys("s"),
		key.WithHelp("s", "submit time"),
	),
	Pause: key.NewBinding(
		key.WithKeys("p"),
		key.WithHelp("p", "pause"),
	),
	Resume: key.NewBinding(
		key.WithKeys("r"),
		key.WithHelp("r", "resume"),
	),
	Cancel: key.NewBinding(
		key.WithKeys("c"),
		key.WithHelp("c", "cancel timer"),
	),
	Up: key.NewBinding(
		key.WithKeys("up"),
		key.WithHelp("↑", "history up"),
	),
	Down: key.NewBinding(
		key.WithKeys("down"),
		key.WithHelp("↓", "history down"),
	),
	Help: key.NewBinding(
		key.WithKeys("?"),
		key.WithHelp("?", "more"),
	),
}

type model struct {
	input       textinput.Model
	message     string
	timerActive bool
	timerPaused bool
	timerStart  time.Time
	timerValue  time.Duration
	pauseTime   time.Time
	totalPaused time.Duration

	history      []string
	historyIndex int
	historyNav   bool
	screen       screen
	help         help.Model
	keys         keyMap

	// For timer recovery
	savedTimerIssue string
	savedTimerValue time.Duration
	lastSaveTime    time.Time
}

func (m model) Init() tea.Cmd {
	m.history = loadHistory()
	m.screen = screenMainApp
	m.help = help.New()
	m.help.Styles.ShortKey = lipgloss.NewStyle().Foreground(colorLightGray)
	m.help.Styles.ShortDesc = lipgloss.NewStyle().Foreground(colorGray)
	m.help.Styles.ShortSeparator = lipgloss.NewStyle().Foreground(colorGray)
	m.help.Styles.FullKey = lipgloss.NewStyle().Foreground(colorLightGray)
	m.help.Styles.FullDesc = lipgloss.NewStyle().Foreground(colorGray)
	m.help.Styles.FullSeparator = lipgloss.NewStyle().Foreground(colorGray)
	m.keys = keys
	return textinput.Blink
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch m.screen {
	case screenMainApp:
		switch msg := msg.(type) {
		case tea.KeyMsg:
			switch msg.String() {
			case "up":
				if !m.timerActive && len(m.history) > 0 {
					if !m.historyNav {
						m.historyIndex = len(m.history) - 1
						m.historyNav = true
					} else if m.historyIndex > 0 {
						m.historyIndex--
					}
					m.input.SetValue(m.history[m.historyIndex])
				}
				return m, nil
			case "down":
				if !m.timerActive && m.historyNav && len(m.history) > 0 {
					if m.historyIndex < len(m.history)-1 {
						m.historyIndex++
						m.input.SetValue(m.history[m.historyIndex])
					} else {
						m.input.SetValue("")
						m.historyNav = false
					}
				}
				return m, nil
			case "ctrl+c", "q":
				return m, tea.Quit
			case "?":
				m.help.ShowAll = !m.help.ShowAll
				return m, nil
			case "enter":
				val := m.input.Value()
				fullId := val
				cfgPath := os.Getenv("HOME") + "/.config/unitrack/unitrack.json"
				b, err := os.ReadFile(cfgPath)
				prefix := "UE"
				if err == nil {
					var cfg apiConfig
					if json.Unmarshal(b, &cfg) == nil && cfg.Prefix != "" {
						prefix = cfg.Prefix
					}
				}
				if !strings.HasPrefix(val, prefix+"-") && val != "" {
					fullId = prefix + "-" + val
				}
				if !m.timerActive && val != "" {
					if saved := loadSavedTimer(fullId); saved != nil {
						m.savedTimerIssue = fullId
						m.savedTimerValue = saved.Duration
						m.screen = screenRecoverTimer
						return m, nil
					}
					found := false
					for _, h := range m.history {
						if h == fullId {
							found = true
							break
						}
					}
					if !found {
						m.history = append(m.history, fullId)
						saveHistory(m.history)
					}
					m.input.SetValue(fullId)
					m.historyNav = false
					m.timerActive = true
					m.timerPaused = false
					m.timerStart = time.Now()
					m.timerValue = 0
					m.totalPaused = 0
					m.message = ""
					m.lastSaveTime = time.Now()
					return m, tickTimer()
				}
				if val == "" && !m.timerActive {
					m.message = "Issue ID cannot be empty."
					return m, nil
				}
			case "p":
				if m.timerActive && !m.timerPaused {
					m.timerPaused = true
					m.pauseTime = time.Now()
					m.message = "Paused. Press 'r' to resume."
					return m, nil
				}
			case "r":
				if m.timerActive && m.timerPaused {
					m.timerPaused = false
					m.totalPaused += time.Since(m.pauseTime)
					m.message = "Resumed."
					return m, tickTimer()
				}
			case "s":
				if m.timerActive {
					ceiled := ceilToQuarter(m.timerValue)
					issueId := m.input.Value()
					cfgPath := os.Getenv("HOME") + "/.config/unitrack/unitrack.json"
					b, err := os.ReadFile(cfgPath)
					prefix := "UE"
					if err == nil {
						var cfg apiConfig
						if json.Unmarshal(b, &cfg) == nil && cfg.Prefix != "" {
							prefix = cfg.Prefix
						}
					}
					if !strings.HasPrefix(issueId, prefix+"-") && issueId != "" {
						issueId = prefix + "-" + issueId
					}
					msg := fmt.Sprintf("Posting %s to Linear for issue %s...", ceiled, issueId)
					m.message = msg
					m.timerActive = false
					m.timerPaused = false
					logEntry := fmt.Sprintf("SUBMIT ISSUE: %s TIME: %s CEIL: %s", issueId, fmtDuration(m.timerValue), ceiled)
					logError(logEntry)
					deleteSavedTimer(issueId)
					go postLinearComment(issueId, ceiled)
					m.input.SetValue("")
					m.input.Focus()
					return m, textinput.Blink
				}
			case "c":
				if m.timerActive {
					m.screen = screenConfirmCancel
					return m, nil
				}
			}
		case timerMsg:
			if m.timerActive && !m.timerPaused {
				m.timerValue = time.Since(m.timerStart) - m.totalPaused
				if time.Since(m.lastSaveTime) >= time.Minute {
					issueId := m.input.Value()
					saveTimer(issueId, m.timerValue, m.timerStart, m.totalPaused)
					m.lastSaveTime = time.Now()
				}
				return m, tickTimer()
			}
		}
		var cmd tea.Cmd
		m.input, cmd = m.input.Update(msg)
		return m, cmd
	case screenConfirmCancel:
		switch msg := msg.(type) {
		case tea.KeyMsg:
			if msg.String() == "y" {
				issueId := m.input.Value()
				deleteSavedTimer(issueId)
				m.timerActive = false
				m.timerPaused = false
				m.screen = screenMainApp
				m.message = "Timer cancelled."
				m.input.SetValue("")
				m.input.Focus()
				return m, textinput.Blink
			} else if msg.String() == "n" {
				m.screen = screenMainApp
				m.message = "Cancel aborted."
				return m, tickTimer()
			}
		}
		return m, nil
	case screenRecoverTimer:
		switch msg := msg.(type) {
		case tea.KeyMsg:
			if msg.String() == "y" {
				// m.input.SetValue(m.savedTimerIssue) // avoid redundant assignment
				m.timerActive = true
				m.timerPaused = false
				m.timerStart = time.Now().Add(-m.savedTimerValue)
				m.timerValue = m.savedTimerValue
				m.totalPaused = 0
				m.message = fmt.Sprintf("Resumed timer at %s", fmtDuration(m.savedTimerValue))
				m.screen = screenMainApp
				m.lastSaveTime = time.Now()
				return m, tickTimer()
			} else if msg.String() == "n" {
				deleteSavedTimer(m.savedTimerIssue)
				// m.input.SetValue(m.savedTimerIssue) // avoid double-set on fresh start
				m.timerActive = true
				m.timerPaused = false
				m.timerStart = time.Now()
				m.timerValue = 0
				m.totalPaused = 0
				m.message = "Starting fresh timer."
				m.screen = screenMainApp
				m.lastSaveTime = time.Now()
				return m, tickTimer()
			}
		}
		return m, nil
	}
	return m, nil
}

func (m model) View() string {
	switch m.screen {
	case screenMainApp:
		logo := logoStyle.Render("⏱ unitrack")
		header := headerBar.Render("Linear time tracker")
		titleLine := lipgloss.JoinHorizontal(lipgloss.Left, logo, header)
		input := inputBox.Render(inputLabel.Render("Issue ID: ") + m.input.View())
		var timer string
		if m.timerActive {
			timer = timerBox.Render("Timer: " + fmtDuration(m.timerValue))
			if m.timerPaused {
				timer += " " + pausedBox.Render("[PAUSED]")
			}
		}
		msg := ""
		if m.message != "" {
			msg = "\n" + msgStyle.Render(m.message)
		}
		helpView := m.help.View(m.keys)
		return lipgloss.JoinVertical(lipgloss.Top,
			titleLine,
			input,
			"",
			timer+msg,
			"",
			helpView,
		)
	case screenConfirmCancel:
		prompt := headerBar.Render("Cancel timer? Press y to confirm, n to abort.")
		return prompt
	case screenRecoverTimer:
		prompt := headerBar.Render(fmt.Sprintf("Found saved timer for %s at %s.", m.savedTimerIssue, fmtDuration(m.savedTimerValue))) +
			headerBar.Render("Continue from saved time? Press y to continue, n to start fresh.")
		return prompt
	}
	return ""
}

func tickTimer() tea.Cmd {
	return tea.Tick(time.Second, func(time.Time) tea.Msg {
		return timerMsg(time.Second)
	})
}

func fmtDuration(d time.Duration) string {
	t := int(d.Seconds())
	h := t / 3600
	m := (t % 3600) / 60
	s := t % 60
	return fmt.Sprintf("%02d:%02d:%02d", h, m, s)
}

func ceilToQuarter(d time.Duration) string {
	tm := d.Minutes()
	quar := int((tm+14.999)/15) * 15
	h := quar / 60
	m := quar % 60
	return fmt.Sprintf("%d:%02d", h, m)
}

type apiConfig struct {
	APIKey          string `json:"api_key"`
	Prefix          string `json:"prefix"`
	TimerExpireDays int    `json:"timer_expire_days,omitempty"`
}

func postLinearComment(issueId, value string) {
	cfgPath := os.Getenv("HOME") + "/.config/unitrack/unitrack.json"
	b, err := os.ReadFile(cfgPath)
	if err != nil {
		logError(fmt.Sprintf("Failed to read config: %v", err))
		return
	}
	var cfg apiConfig
	err = json.Unmarshal(b, &cfg)
	if err != nil || cfg.APIKey == "" {
		logError(fmt.Sprintf("Failed to parse config or missing key: %v", err))
		return
	}
	client := resty.New()
	mutation := `mutation CommentCreate { commentCreate(input: { issueId: "` + issueId + `", body: "` + value + `" }) { comment { id } } }`
	resp, err := client.R().
		SetHeader("Authorization", cfg.APIKey).
		SetHeader("Content-Type", "application/json").
		SetBody(map[string]string{"query": mutation}).
		Post("https://api.linear.app/graphql")
	logError(fmt.Sprintf("Linear API response status: %d, response: %s", resp.StatusCode(), resp.String()))
	if err != nil {
		logError(fmt.Sprintf("Linear API error: %v", err))
		return
	}
	if resp.StatusCode() != 200 {
		logError(fmt.Sprintf("Linear API returned non-200: %d. Response: %s", resp.StatusCode(), resp.String()))
	}
}

func logError(msg string) {
	logPath := os.Getenv("HOME") + "/.config/unitrack/unitrack.log"
	f, ferr := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if ferr != nil {
		fmt.Fprintf(os.Stderr, "Could not log error: %v\nOriginal error: %s\n", ferr, msg)
		return
	}
	defer f.Close()
	f.WriteString(time.Now().Format(time.RFC3339) + " " + msg + "\n")
}

func loadHistory() []string {
	path := os.Getenv("HOME") + "/.config/unitrack/history"
	b, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	lines := strings.Split(string(b), "\n")
	var out []string
	for _, l := range lines {
		l = strings.TrimSpace(l)
		if l != "" {
			out = append(out, l)
		}
	}
	return out
}

func saveHistory(hist []string) {
	path := os.Getenv("HOME") + "/.config/unitrack/history"
	_ = os.MkdirAll(os.Getenv("HOME")+"/.config/unitrack", 0700)
	uniq := make(map[string]bool)
	var order []string
	for _, h := range hist {
		if h != "" && !uniq[h] {
			uniq[h] = true
			order = append(order, h)
		}
	}
	os.WriteFile(path, []byte(strings.Join(order, "\n")), 0600)
}

type savedTimer struct {
	IssueID     string        `json:"issue_id"`
	Duration    time.Duration `json:"duration"`
	StartTime   time.Time     `json:"start_time"`
	TotalPaused time.Duration `json:"total_paused"`
	SavedAt     time.Time     `json:"saved_at"`
}

func saveTimer(issueID string, duration time.Duration, startTime time.Time, totalPaused time.Duration) {
	saved := savedTimer{
		IssueID:     issueID,
		Duration:    duration,
		StartTime:   startTime,
		TotalPaused: totalPaused,
		SavedAt:     time.Now(),
	}
	path := os.Getenv("HOME") + "/.config/unitrack/saved_timer_" + strings.ReplaceAll(issueID, "/", "_") + ".json"
	b, err := json.MarshalIndent(saved, "", "  ")
	if err != nil {
		logError(fmt.Sprintf("Failed to marshal saved timer: %v", err))
		return
	}
	err = os.WriteFile(path, b, 0600)
	if err != nil {
		logError(fmt.Sprintf("Failed to save timer: %v", err))
	}
}

func loadSavedTimer(issueID string) *savedTimer {
	path := os.Getenv("HOME") + "/.config/unitrack/saved_timer_" + strings.ReplaceAll(issueID, "/", "_") + ".json"
	b, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var saved savedTimer
	err = json.Unmarshal(b, &saved)
	if err != nil {
		logError(fmt.Sprintf("Failed to unmarshal saved timer: %v", err))
		return nil
	}

	expireDays := 5
	cfgPath := os.Getenv("HOME") + "/.config/unitrack/unitrack.json"
	b, err = os.ReadFile(cfgPath)
	if err == nil {
		var cfg apiConfig
		if json.Unmarshal(b, &cfg) == nil && cfg.TimerExpireDays > 0 {
			expireDays = cfg.TimerExpireDays
		}
	}
	if time.Since(saved.SavedAt) > time.Duration(expireDays)*24*time.Hour {
		deleteSavedTimer(issueID)
		return nil
	}
	return &saved
}

func deleteSavedTimer(issueID string) {
	path := os.Getenv("HOME") + "/.config/unitrack/saved_timer_" + strings.ReplaceAll(issueID, "/", "_") + ".json"
	os.Remove(path)
}

func main() {
	if len(os.Args) > 1 && os.Args[1] == "--version" {
		fmt.Printf("unitrack %s\n", version)
		return
	}

	cfgPath := os.Getenv("HOME") + "/.config/unitrack/unitrack.json"
	b, err := os.ReadFile(cfgPath)
	prefix := "UE"
	if err == nil {
		var cfg apiConfig
		if json.Unmarshal(b, &cfg) == nil && cfg.Prefix != "" {
			prefix = cfg.Prefix
		}
	}
	input := textinput.New()
	input.Placeholder = prefix + "-1234"
	input.CharLimit = 50
	input.Width = 20
	input.Focus()
	helpModel := help.New()
	helpModel.Styles.ShortKey = lipgloss.NewStyle().Foreground(colorLightGray)
	helpModel.Styles.ShortDesc = lipgloss.NewStyle().Foreground(colorGray)
	helpModel.Styles.ShortSeparator = lipgloss.NewStyle().Foreground(colorGray)
	helpModel.Styles.FullKey = lipgloss.NewStyle().Foreground(colorLightGray)
	helpModel.Styles.FullDesc = lipgloss.NewStyle().Foreground(colorGray)
	helpModel.Styles.FullSeparator = lipgloss.NewStyle().Foreground(colorGray)
	m := model{
		input:   input,
		message: "",
		help:    helpModel,
		keys:    keys,
	}
	m.history = loadHistory()
	p := tea.NewProgram(m)
	if err := p.Start(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
