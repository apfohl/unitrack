package main

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	textinput "github.com/charmbracelet/bubbles/textinput"
	resty "github.com/go-resty/resty/v2"
)

type timerMsg time.Duration

type model struct {
	input         textinput.Model
	message       string
	timerActive   bool
	timerStart    time.Time
	timerValue    time.Duration
}

func (m model) Init() tea.Cmd {
	return textinput.Blink
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit
		case "enter":
			m.timerActive = true
			m.timerStart = time.Now()
			m.timerValue = 0
			m.message = ""
			return m, tickTimer()
		case "s":
			if m.timerActive {
				ceiled := ceilToQuarter(m.timerValue)
				issueId := m.input.Value()
				msg := fmt.Sprintf("Posting %s to Linear for issue %s...", ceiled, issueId)
				m.message = msg
				m.timerActive = false
				go postLinearComment(issueId, ceiled)
				m.input.SetValue("")
				m.input.Focus()
				return m, textinput.Blink
			}
		}
	case timerMsg:
		if m.timerActive {
			m.timerValue = time.Since(m.timerStart)
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
	}
	if m.message != "" {
		view += m.message + "\n"
	}
	view += "Press q or ctrl+c to quit. Press s to submit time."
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
	return fmt.Sprintf("%02d:%02d", h, m)
}

type apiConfig struct {
	APIKey string `json:"api_key"`
}

func postLinearComment(issueId, value string) {
	cfgPath := os.Getenv("HOME") + "/.config/unitrack/unitrack.json"
	b, err := os.ReadFile(cfgPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to read config: %v\n", err)
		return
	}
	var cfg apiConfig
	err = json.Unmarshal(b, &cfg)
	if err != nil || cfg.APIKey == "" {
		fmt.Fprintf(os.Stderr, "Failed to parse config or missing key: %v\n", err)
		return
	}
	client := resty.New()
	mutation := `mutation CommentCreate { commentCreate(input: { issueId: "` + issueId + `", body: "` + value + `" }) { comment { id } } }`
	resp, err := client.R().
		SetHeader("Authorization", cfg.APIKey).
		SetHeader("Content-Type", "application/json").
		SetBody(map[string]string{"query": mutation}).
		Post("https://api.linear.app/graphql")
	if err != nil {
		logError(fmt.Sprintf("Linear API error: %v", err))
		return
	}
	if resp.StatusCode() != 200 {
		logError(fmt.Sprintf("Linear API returned non-200: %d. Response: %s", resp.StatusCode(), resp.String()))
	}
}

func logError(msg string) {
	f, ferr := os.OpenFile("unitrack_error.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
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
	p := tea.NewProgram(m)
	if err := p.Start(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
