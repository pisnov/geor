package ui

import (
	"fmt"
	"os"
	"strings"
	"time"

	"yt-audio/player"

	tea "github.com/charmbracelet/bubbletea"
)

const progressBarWidth = 28

type tickMsg time.Time

func tickCmd() tea.Cmd {
	return tea.Tick(time.Second, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

type model struct {
	player    *player.Player
	urlInput  string
	statusMsg string
	hasTrack  bool
	playing   bool
	speed     float64
}

func initialModel(p *player.Player) model {
	return model{player: p, speed: 1.0}
}

func (m model) Init() tea.Cmd {
	return tickCmd()
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tickMsg:
		return m, tickCmd()
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit
		case "enter":
			if !m.playing && strings.TrimSpace(m.urlInput) != "" {
				m.statusMsg = "Loading..."
				err := m.player.Play(m.urlInput)
				if err != nil {
					m.statusMsg = "Error: " + err.Error()
				} else {
					m.statusMsg = ""
					m.playing = true
					m.hasTrack = true
					m.speed = 1.0
					m.urlInput = ""
				}
			}
		case "+":
			if m.playing || m.hasTrack {
				m.speed += 0.25
				m.player.SetSpeed(m.speed)
			}
		case "-":
			if (m.playing || m.hasTrack) && m.speed > 0.25 {
				m.speed -= 0.25
				m.player.SetSpeed(m.speed)
			}
		case "r":
			if m.playing || m.hasTrack {
				m.speed = 1.0
				m.player.SetSpeed(1.0)
			}
		case "p":
			if m.playing {
				m.player.Pause()
				m.playing = false
				m.statusMsg = "Paused"
			} else if m.hasTrack {
				m.player.Resume()
				m.playing = true
				m.statusMsg = ""
			}
		case "backspace":
			if !m.playing && len(m.urlInput) > 0 {
				m.urlInput = m.urlInput[:len(m.urlInput)-1]
			}
		default:
			if !m.playing {
				if msg.Paste {
					m.urlInput += string(msg.Runes)
				} else {
					m.urlInput += msg.String()
				}
			}
		}
	case tea.WindowSizeMsg:
		// ignore
	}
	return m, nil
}

func progressBar(pos, total time.Duration, width int) string {
	if total <= 0 || width <= 0 {
		return strings.Repeat("░", width)
	}
	pct := float64(pos) / float64(total)
	if pct > 1.0 {
		pct = 1.0
	}
	filled := int(pct * float64(width))
	return strings.Repeat("█", filled) + strings.Repeat("░", width-filled)
}

func fmtDur(d time.Duration) string {
	d = d.Round(time.Second)
	return fmt.Sprintf("%02d:%02d", int(d.Minutes()), int(d.Seconds())%60)
}

func (m model) View() string {
	sep := strings.Repeat("─", 42)

	title := "(no track loaded)"
	if m.player.Title != "" {
		runes := []rune(m.player.Title)
		if len(runes) > 44 {
			runes = append(runes[:41], []rune("...")...)
		}
		title = string(runes)
	}

	pos := m.player.GetPosition()
	total := m.player.Length
	bar := progressBar(pos, total, progressBarWidth)
	posStr := fmtDur(pos)
	totalStr := ""
	if total > 0 {
		totalStr = " / " + fmtDur(total)
	}

	stateIcon := "▶"
	if !m.playing {
		stateIcon = "⏸"
	}

	bottom := ""
	if !m.playing {
		bottom = "  URL: " + m.urlInput + "_\n"
	}
	if m.statusMsg != "" {
		bottom += "  " + m.statusMsg + "\n"
	}

	return "\n  🎧 YT Audio Player\n" +
		"  " + sep + "\n" +
		"  " + stateIcon + "  " + title + "\n\n" +
		"  [" + bar + "]\n" +
		"  " + posStr + totalStr + "\n\n" +
		fmt.Sprintf("  Speed: %.2fx\n", m.speed) +
		"  " + sep + "\n" +
		"  [Enter] Play  [p] Pause/Resume  [+/-] Speed  [r] Reset  [q] Quit\n" +
		bottom
}

func StartTUI(p *player.Player) {
	m := initialModel(p)
	if _, err := tea.NewProgram(m).Run(); err != nil {
		fmt.Println("Error running TUI:", err)
		os.Exit(1)
	}
}
