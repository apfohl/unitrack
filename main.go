package main

import (
	"encoding/json"
	"fmt"
	"os"
	"time"
	"strings"
	tea "github.com/charmbracelet/bubbletea"
	textinput "github.com/charmbracelet/bubbles/textinput"
	resty "github.com/go-resty/resty/v2"
)

type timerMsg time.Duration

type model struct {
	input         textinput.Model
	message       string
	timerActive   bool
	timerPaused   bool
	timerStart    time.Time
	timerValue    time.Duration
	pauseTime     time.Time
	totalPaused   time.Duration

	history       []string
	historyIndex  int
	historyNav    bool
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
	// write unique vals only, most recent last
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

func (m model) Init() tea.Cmd {
	m.history = loadHistory()
	return textinput.Blink
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		// navigation for history when not running a timer
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
		case "enter":
			if val := m.input.Value(); !m.timerActive && val != "" {
				// add unique, non-empty to history if new
				found := false
				for _, h := range m.history {
					if h == val {
						found = true
						break
					}
				}
				if !found {
					m.history = append(m.history, val)
					saveHistory(m.history)
				}
				m.historyNav = false
			}
			m.timerActive = true
			m.timerPaused = false
			m.timerStart = time.Now()
			m.timerValue = 0
			m.totalPaused = 0
			m.message = ""
			return m, tickTimer()
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
				msg := fmt.Sprintf("Posting %s to Linear for issue %s...", ceiled, issueId)
				m.message = msg
				m.timerActive = false
				m.timerPaused = false
				logEntry := fmt.Sprintf("SUBMIT ISSUE: %s TIME: %s CEIL: %s", issueId, fmtDuration(m.timerValue), ceiled)
				logError(logEntry)
				go postLinearComment(issueId, ceiled)
				m.input.SetValue("")
				m.input.Focus()
				return m, textinput.Blink
			}
		case "c":
			if m.timerActive {
				m.timerActive = false
				m.timerPaused = false
				m.message = "Timer cancelled."
				m.input.SetValue("")
				m.input.Focus()
				return m, textinput.Blink
			}
		}
	case timerMsg:
		if m.timerActive && !m.timerPaused {
			m.timerValue = time.Since(m.timerStart) - m.totalPaused
			return m, tickTimer()
		}
	}
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	return m, cmd
}

func (m model) View() string {
	view := "Enter issue ID: " + m.input.View() + "\n"
	if m.timerActive {
		view += fmt.Sprintf("Timer: %s\n", fmtDuration(m.timerValue))
		if m.timerPaused {
			view += "[PAUSED]\n"
		}
	}
	if m.message != "" {
		view += m.message + "\n"
	}
	view += "Press q or ctrl+c to quit. 's': submit, 'p': pause, 'r': resume, 'c': cancel. Up/down to cycle history."
	return view
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
	quar := int((tm+14.999)/15) * 15 // ceil to next 15
	h := quar / 60
	m := quar % 60
	return fmt.Sprintf("%d:%02d", h, m)
}

type apiConfig struct {
	APIKey string `json:"api_key"`
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

func main() {
	input := textinput.New()
	input.Placeholder = "Issue ID"
	input.CharLimit = 20
	input.Focus()
	m := model{
		input:   input,
		message: "",
	}
	m.history = loadHistory()
	p := tea.NewProgram(m)
	if err := p.Start(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
