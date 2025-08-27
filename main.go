package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/progress"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/go-resty/resty/v2"
)

var version = "unknown"

var (
	colorRed       lipgloss.Color
	colorYellow    lipgloss.Color
	colorOrange    lipgloss.Color
	colorLightGray lipgloss.Color
	colorGray      lipgloss.Color

	logoStyle lipgloss.Style
	headerBar lipgloss.Style

	inputLabel   lipgloss.Style
	timerBox     lipgloss.Style
	progressBox  lipgloss.Style
	spinnerStyle lipgloss.Style
	pausedBox    lipgloss.Style
	msgStyle     lipgloss.Style
	helpStyle    lipgloss.Style
)

func initializeTheme(theme string) {
	if theme == "light" {
		colorRed = lipgloss.Color("124")       // darker red for light backgrounds
		colorYellow = lipgloss.Color("130")    // darker yellow/orange for light backgrounds
		colorOrange = lipgloss.Color("208")    // darker orange for spinner
		colorLightGray = lipgloss.Color("240") // darker gray for keys
		colorGray = lipgloss.Color("235")      // darker gray for descriptions
	} else {
		colorRed = lipgloss.Color("131")       // muted red for dark backgrounds
		colorYellow = lipgloss.Color("143")    // muted yellow for dark backgrounds
		colorOrange = lipgloss.Color("166")    // orange for spinner
		colorLightGray = lipgloss.Color("250") // lighter gray for keys
		colorGray = lipgloss.Color("245")      // gray for descriptions
	}

	logoStyle = lipgloss.NewStyle().Foreground(colorRed).Bold(true).Padding(1, 0, 1, 1)
	headerBar = lipgloss.NewStyle().Bold(true).Padding(1, 1).Padding(1, 0, 1, 2)
	inputLabel = lipgloss.NewStyle().Foreground(colorYellow).Bold(true).PaddingLeft(1)
	timerBox = lipgloss.NewStyle().Foreground(colorYellow).Bold(true).PaddingLeft(1).PaddingTop(1)
	progressBox = lipgloss.NewStyle().PaddingLeft(1).PaddingTop(1)
	spinnerStyle = lipgloss.NewStyle().PaddingLeft(1).PaddingTop(1)
	pausedBox = lipgloss.NewStyle().Foreground(colorRed).Bold(true).Underline(true).PaddingLeft(2).PaddingTop(1)
	msgStyle = lipgloss.NewStyle().Foreground(colorRed).Italic(true).PaddingLeft(1).PaddingTop(1)
	helpStyle = lipgloss.NewStyle().PaddingLeft(1).PaddingTop(1)
}

type timerMsg time.Duration

type screen int

const (
	screenMain screen = iota
	screenConfirmCancel
	screenRecoverTimer
	screenLimitedTimerSetup
)

type keyMap struct {
	Quit         key.Binding
	Start        key.Binding
	Submit       key.Binding
	Pause        key.Binding
	Resume       key.Binding
	Cancel       key.Binding
	Up           key.Binding
	Down         key.Binding
	Help         key.Binding
	LimitedTimer key.Binding
}

func (k keyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Start, k.LimitedTimer, k.Submit, k.Help, k.Quit}
}

func (k keyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.Start, k.LimitedTimer, k.Submit, k.Pause, k.Resume},
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
	LimitedTimer: key.NewBinding(
		key.WithKeys("l"),
		key.WithHelp("l", "limited timer"),
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
	spinner      spinner.Model

	// For timer recovery
	savedTimerIssue   string
	savedTimerValue   time.Duration
	savedTimerLimited bool
	savedTimerLimit   time.Duration
	lastSaveTime      time.Time

	// For limited timer
	limitedTimer   bool
	timerLimit     time.Duration
	limitInput     textinput.Model
	progressBar    progress.Model
	pendingIssueID string
}

func (m model) Init() tea.Cmd {
	theme := "dark"
	b, err := os.ReadFile(os.Getenv("HOME") + "/.config/unitrack/unitrack.json")
	if err == nil {
		var cfg apiConfig
		if json.Unmarshal(b, &cfg) == nil && cfg.Theme != "" {
			theme = cfg.Theme
		}
	}
	initializeTheme(theme)

	m.history = loadHistory()
	m.screen = screenMain
	m.keys = keys

	m.help = help.New()
	m.help.Styles.ShortKey = lipgloss.NewStyle().Foreground(colorLightGray)
	m.help.Styles.ShortDesc = lipgloss.NewStyle().Foreground(colorGray)
	m.help.Styles.ShortSeparator = lipgloss.NewStyle().Foreground(colorGray)
	m.help.Styles.FullKey = lipgloss.NewStyle().Foreground(colorLightGray)
	m.help.Styles.FullDesc = lipgloss.NewStyle().Foreground(colorGray)
	m.help.Styles.FullSeparator = lipgloss.NewStyle().Foreground(colorGray)

	m.spinner = spinner.New()
	m.spinner.Spinner = spinner.Dot
	m.spinner.Style = lipgloss.NewStyle().Foreground(colorOrange)

	m.limitInput = textinput.New()
	m.limitInput.Placeholder = "15"
	m.limitInput.CharLimit = 4
	m.limitInput.Width = 10

	m.progressBar = progress.New(progress.WithDefaultGradient())
	m.progressBar.Width = 40

	return textinput.Blink
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch m.screen {
	case screenMain:
		switch message := msg.(type) {
		case tea.KeyMsg:
			switch message.String() {
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

			case "l":
				val := m.input.Value()
				fullId := val

				b, err := os.ReadFile(os.Getenv("HOME") + "/.config/unitrack/unitrack.json")
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
					m.pendingIssueID = fullId
					m.screen = screenLimitedTimerSetup
					m.limitInput.Focus()

					return m, textinput.Blink
				}

				if val == "" && !m.timerActive {
					m.message = "Issue ID cannot be empty."

					return m, nil
				}

			case "p":
				if m.timerActive && !m.timerPaused {
					m.timerPaused = true
					m.pauseTime = time.Now()
					m.message = "Press 'r' to resume."

					return m, nil
				}

			case "r":
				if m.timerActive && m.timerPaused {
					m.timerPaused = false
					m.totalPaused += time.Since(m.pauseTime)
					m.message = "Timer resumed."

					if m.limitedTimer {
						return m, tickTimer()
					} else {
						return m, tea.Batch(tickTimer(), m.spinner.Tick)
					}
				}

			case "s":
				if m.timerActive {
					ceiled := ceilToQuarter(m.timerValue)
					issueId := m.input.Value()

					b, err := os.ReadFile(os.Getenv("HOME") + "/.config/unitrack/unitrack.json")
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

					m.message = fmt.Sprintf("Posting %s to Linear for issue %s...", ceiled, issueId)
					m.timerActive = false
					m.timerPaused = false

					logError(fmt.Sprintf(
						"SUBMIT ISSUE: %s TIME: %s CEIL: %s",
						issueId,
						fmtDuration(m.timerValue),
						ceiled,
					))

					deleteSavedTimer(issueId)

					go postLinearComment(issueId, ceiled)

					m.history = loadHistory()
					m.input.SetValue("")
					m.input.Focus()

					return m, textinput.Blink
				}

			case "c":
				if m.timerActive {
					m.screen = screenConfirmCancel
					return m, nil
				}

			case "enter":
				val := m.input.Value()
				fullId := val

				b, err := os.ReadFile(os.Getenv("HOME") + "/.config/unitrack/unitrack.json")
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
						m.savedTimerLimited = saved.LimitedTimer
						m.savedTimerLimit = saved.TimerLimit
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
					m.input.Blur()
					m.timerValue = 0
					m.totalPaused = 0
					m.message = "Timer started."
					m.lastSaveTime = time.Now()

					return m, tea.Batch(tickTimer(), m.spinner.Tick)
				}

				if val == "" && !m.timerActive {
					m.message = "Issue ID cannot be empty."

					return m, nil
				}
			}

		case timerMsg:
			if m.timerActive && !m.timerPaused {
				m.timerValue = time.Since(m.timerStart) - m.totalPaused
				if m.limitedTimer && m.timerValue >= m.timerLimit {
					m.timerValue = m.timerLimit
					ceiled := ceilToQuarter(m.timerValue)
					issueId := m.input.Value()
					m.message = fmt.Sprintf("Time limit reached! Posting %s to Linear for issue %s...", ceiled, issueId)
					m.timerActive = false
					m.timerPaused = false
					m.limitedTimer = false
					logEntry := fmt.Sprintf("AUTO-SUBMIT ISSUE: %s TIME: %s CEIL: %s", issueId, fmtDuration(m.timerValue), ceiled)
					logError(logEntry)
					deleteSavedTimer(issueId)
					go postLinearComment(issueId, ceiled)
					go showTimerNotification(issueId, ceiled)
					m.history = loadHistory()
					m.input.SetValue("")
					m.input.Focus()

					return m, textinput.Blink
				}

				if time.Since(m.lastSaveTime) >= time.Minute {
					issueId := m.input.Value()
					saveTimer(issueId, m.timerValue, m.timerStart, m.totalPaused, m.limitedTimer, m.timerLimit)
					m.lastSaveTime = time.Now()
				}

				return m, tea.Batch(tickTimer(), m.spinner.Tick)
			}
		}

		var cmd tea.Cmd
		if !m.timerActive {
			m.input, cmd = m.input.Update(msg)
		}
		m.spinner, _ = m.spinner.Update(msg)

		return m, cmd

	case screenConfirmCancel:
		switch message := msg.(type) {
		case tea.KeyMsg:
			if message.String() == "y" {
				issueId := m.input.Value()
				deleteSavedTimer(issueId)
				m.timerActive = false
				m.timerPaused = false
				m.limitedTimer = false
				m.limitedTimer = false
				m.screen = screenMain
				m.message = "Timer cancelled."
				m.input.SetValue("")
				m.input.Focus()

				return m, textinput.Blink
			} else if message.String() == "n" {
				m.screen = screenMain
				m.message = "Cancel aborted."

				return m, tea.Batch(tickTimer(), m.spinner.Tick)
			}
		}

		return m, nil

	case screenRecoverTimer:
		switch message := msg.(type) {
		case tea.KeyMsg:
			if message.String() == "y" {
				m.timerActive = true
				m.timerPaused = false
				m.limitedTimer = m.savedTimerLimited
				m.timerLimit = m.savedTimerLimit
				m.timerStart = time.Now().Add(-m.savedTimerValue)
				m.input.Blur()
				m.timerValue = m.savedTimerValue
				m.totalPaused = 0
				m.message = fmt.Sprintf("Resumed timer at %s", fmtDuration(m.savedTimerValue))
				m.screen = screenMain
				m.lastSaveTime = time.Now()

				return m, tea.Batch(tickTimer(), m.spinner.Tick)
			} else if message.String() == "n" {
				deleteSavedTimer(m.savedTimerIssue)
				m.timerActive = true
				m.timerPaused = false
				m.timerStart = time.Now()
				m.input.Blur()
				m.timerValue = 0
				m.totalPaused = 0
				m.message = "Starting fresh timer."
				m.screen = screenMain
				m.lastSaveTime = time.Now()

				return m, tea.Batch(tickTimer(), m.spinner.Tick)
			}
		}

		return m, nil

	case screenLimitedTimerSetup:
		switch message := msg.(type) {
		case tea.KeyMsg:
			switch message.String() {
			case "enter":
				minutesStr := m.limitInput.Value()
				if minutesStr == "" {
					m.message = "Please enter a number of minutes."
					return m, nil
				}
				minutes, err := strconv.Atoi(minutesStr)
				if err != nil || minutes <= 0 {
					m.message = "Please enter a valid positive number."
					return m, nil
				}
				m.timerLimit = time.Duration(minutes) * time.Minute
				m.limitedTimer = true
				found := false
				for _, h := range m.history {
					if h == m.pendingIssueID {
						found = true
						break
					}
				}
				if !found {
					m.history = append(m.history, m.pendingIssueID)
					saveHistory(m.history)
				}
				m.input.SetValue(m.pendingIssueID)
				m.historyNav = false
				m.timerActive = true
				m.timerPaused = false
				m.timerStart = time.Now()
				m.input.Blur()
				m.timerValue = 0
				m.totalPaused = 0
				m.message = fmt.Sprintf("Limited timer started for %d minutes", minutes)
				m.screen = screenMain
				return m, tea.Batch(tickTimer(), m.spinner.Tick)

			case "ctrl+c", "q", "esc":
				m.screen = screenMain
				m.limitInput.SetValue("")
				m.message = "Limited timer cancelled."
				return m, nil
			}
		}

		var cmd tea.Cmd
		m.limitInput, cmd = m.limitInput.Update(msg)

		return m, cmd
	}

	return m, nil
}

func (m model) View() string {
	switch m.screen {
	case screenMain:
		titleLine := lipgloss.JoinHorizontal(
			lipgloss.Left,
			logoStyle.Render("⏱ unitrack"),
			headerBar.Render("Linear time tracker"),
		)
		input := lipgloss.JoinHorizontal(
			lipgloss.Left,
			inputLabel.Render("Issue ID: "),
			m.input.View(),
		)
		shortcutsHelp := helpStyle.Render(m.help.View(keys))

		var timer string
		if m.timerActive {
			if m.limitedTimer {
				timerProgress := float64(m.timerValue) / float64(m.timerLimit)
				if timerProgress > 1.0 {
					timerProgress = 1.0
				}

				if m.timerPaused {
					timer = lipgloss.JoinVertical(
						lipgloss.Top,
						lipgloss.JoinHorizontal(
							lipgloss.Left,
							timerBox.Render("Timer: "+fmtDuration(m.timerValue)),
							pausedBox.Render("[PAUSED]"),
						),
						progressBox.Render(m.progressBar.ViewAs(timerProgress)),
						msgStyle.Render(fmt.Sprintf("Limit: %s", fmtDuration(m.timerLimit))),
					)
				} else {
					timer = lipgloss.JoinVertical(
						lipgloss.Top,
						lipgloss.JoinHorizontal(
							lipgloss.Left,
							spinnerStyle.Render(m.spinner.View()),
							timerBox.Render("Timer: "+fmtDuration(m.timerValue)),
						),
						progressBox.Render(m.progressBar.ViewAs(timerProgress)),
						msgStyle.Render(fmt.Sprintf("Limit: %s", fmtDuration(m.timerLimit))),
					)
				}
			} else {
				if m.timerPaused {
					timer = lipgloss.JoinHorizontal(
						lipgloss.Left,
						timerBox.Render("Timer: "+fmtDuration(m.timerValue)),
						pausedBox.Render("[PAUSED]"),
					)
				} else {
					timer = lipgloss.JoinHorizontal(
						lipgloss.Left,
						spinnerStyle.Render(m.spinner.View()),
						timerBox.Render("Timer: "+fmtDuration(m.timerValue)),
					)
				}
			}

			return lipgloss.JoinVertical(
				lipgloss.Top,
				titleLine,
				input,
				timer,
				msgStyle.Render(m.message),
				shortcutsHelp,
			)
		}

		return lipgloss.JoinVertical(
			lipgloss.Top,
			titleLine,
			input,
			msgStyle.Render(m.message),
			shortcutsHelp,
		)

	case screenConfirmCancel:
		return headerBar.Render("Cancel timer? Press y to confirm, n to abort.")

	case screenRecoverTimer:
		var timerInfo string
		if m.savedTimerLimited {
			limitMinutes := int(m.savedTimerLimit.Minutes())
			timerInfo = fmt.Sprintf(
				"Found saved LIMITED timer for %s at %s (limit: %d minutes).",
				m.savedTimerIssue,
				fmtDuration(m.savedTimerValue),
				limitMinutes,
			)
		} else {
			timerInfo = fmt.Sprintf(
				"Found saved timer for %s at %s.",
				m.savedTimerIssue,
				fmtDuration(m.savedTimerValue),
			)
		}

		return lipgloss.JoinHorizontal(
			lipgloss.Top,
			headerBar.Render(timerInfo),
			headerBar.Render("Continue from saved time? Press y to continue, n to start fresh."),
		)

	case screenLimitedTimerSetup:
		return lipgloss.JoinVertical(
			lipgloss.Top,
			lipgloss.JoinHorizontal(
				lipgloss.Left,
				logoStyle.Render("⏱ unitrack"),
				headerBar.Render("Limited Timer Setup"),
			),
			inputLabel.Render(fmt.Sprintf("Issue: %s", m.pendingIssueID)),
			lipgloss.JoinHorizontal(
				lipgloss.Left,
				inputLabel.Render("Minutes: "),
				m.limitInput.View(),
			),
			lipgloss.JoinHorizontal(
				lipgloss.Left,
				msgStyle.Render("Enter the number of minutes for the timer limit, then press Enter."),
				msgStyle.Render(m.message),
			),
		)
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
	Theme           string `json:"theme,omitempty"`
}

func postLinearComment(issueId, value string) {
	b, err := os.ReadFile(os.Getenv("HOME") + "/.config/unitrack/unitrack.json")
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

	mutation := `mutation CommentCreate { commentCreate(input: { issueId: "` + issueId + `", body: "` + value + `" }) { comment { id } } }`
	resp, err := resty.New().R().
		SetHeader("Authorization", cfg.APIKey).
		SetHeader("Content-Type", "application/json").
		SetBody(map[string]string{"query": mutation}).
		Post("https://api.linear.app/graphql")

	if resp == nil {
		logError("Linear API response is nil")
		return
	}

	logError(fmt.Sprintf("Linear API response status: %d, response: %s", resp.StatusCode(), resp.String()))

	if err != nil {
		logError(fmt.Sprintf("Linear API error: %v", err))
		return
	}

	if resp.StatusCode() != 200 {
		logError(fmt.Sprintf("Linear API returned non-200: %d. Response: %s", resp.StatusCode(), resp.String()))
	}
}

func showTimerNotification(issueId, timeValue string) {
	msg := fmt.Sprintf("Timer for %s completed. Time logged: %s", issueId, timeValue)
	fmt.Printf("\x1b]9;%s\x1b\\", msg)
}

func logError(msg string) {
	logPath := os.Getenv("HOME") + "/.config/unitrack/unitrack.log"

	f, ferr := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if ferr != nil {
		_, _ = fmt.Fprintf(os.Stderr, "Could not log error: %v\nOriginal error: %s\n", ferr, msg)
		return
	}
	defer func(f *os.File) {
		err := f.Close()
		if err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "Could not close log file: %v\n", err)
		}
	}(f)

	_, _ = f.WriteString(time.Now().Format(time.RFC3339) + " " + msg + "\n")
}

func loadHistory() []string {
	b, err := os.ReadFile(os.Getenv("HOME") + "/.config/unitrack/history")
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
	_ = os.MkdirAll(os.Getenv("HOME")+"/.config/unitrack", 0700)

	uniq := make(map[string]bool)
	var order []string
	for _, h := range hist {
		if h != "" && !uniq[h] {
			uniq[h] = true
			order = append(order, h)
		}
	}

	_ = os.WriteFile(os.Getenv("HOME")+"/.config/unitrack/history", []byte(strings.Join(order, "\n")), 0600)
}

type savedTimer struct {
	IssueID      string        `json:"issue_id"`
	Duration     time.Duration `json:"duration"`
	StartTime    time.Time     `json:"start_time"`
	TotalPaused  time.Duration `json:"total_paused"`
	SavedAt      time.Time     `json:"saved_at"`
	LimitedTimer bool          `json:"limited_timer"`
	TimerLimit   time.Duration `json:"timer_limit"`
}

func saveTimer(
	issueID string,
	duration time.Duration,
	startTime time.Time,
	totalPaused time.Duration,
	limitedTimer bool,
	timerLimit time.Duration,
) {
	saved := savedTimer{
		IssueID:      issueID,
		Duration:     duration,
		StartTime:    startTime,
		TotalPaused:  totalPaused,
		SavedAt:      time.Now(),
		LimitedTimer: limitedTimer,
		TimerLimit:   timerLimit,
	}

	b, err := json.MarshalIndent(saved, "", "  ")
	if err != nil {
		logError(fmt.Sprintf("Failed to marshal saved timer: %v", err))
		return
	}

	err = os.WriteFile(
		os.Getenv("HOME")+"/.config/unitrack/saved_timer_"+strings.ReplaceAll(issueID, "/", "_")+".json",
		b,
		0600,
	)
	if err != nil {
		logError(fmt.Sprintf("Failed to save timer: %v", err))
	}
}

func loadSavedTimer(issueID string) *savedTimer {
	b, err := os.ReadFile(
		os.Getenv("HOME") + "/.config/unitrack/saved_timer_" + strings.ReplaceAll(issueID, "/", "_") + ".json",
	)
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

	b, err = os.ReadFile(os.Getenv("HOME") + "/.config/unitrack/unitrack.json")
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
	_ = os.Remove(
		os.Getenv("HOME") + "/.config/unitrack/saved_timer_" + strings.ReplaceAll(issueID, "/", "_") + ".json",
	)
}

func main() {
	if len(os.Args) > 1 && os.Args[1] == "--version" {
		fmt.Printf("unitrack %s\n", version)
		return
	}

	theme := "dark"
	prefix := "UE"
	b, err := os.ReadFile(os.Getenv("HOME") + "/.config/unitrack/unitrack.json")
	if err == nil {
		var cfg apiConfig
		if json.Unmarshal(b, &cfg) == nil {
			if cfg.Theme != "" {
				theme = cfg.Theme
			}
			if cfg.Prefix != "" {
				prefix = cfg.Prefix
			}
		}
	}
	initializeTheme(theme)

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

	spinnerModel := spinner.New()
	spinnerModel.Spinner = spinner.Dot
	spinnerModel.Style = lipgloss.NewStyle().Foreground(colorOrange)

	limitInput := textinput.New()
	limitInput.Placeholder = "15"
	limitInput.CharLimit = 4
	limitInput.Width = 10

	progressBar := progress.New(progress.WithDefaultGradient())
	progressBar.Width = 40

	m := model{
		input:       input,
		message:     "Enter issue ID and hit 'enter' to start timer or 'l' to set up limited timer.",
		help:        helpModel,
		keys:        keys,
		spinner:     spinnerModel,
		limitInput:  limitInput,
		progressBar: progressBar,
	}

	m.history = loadHistory()

	if _, err = tea.NewProgram(m, tea.WithAltScreen()).Run(); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
