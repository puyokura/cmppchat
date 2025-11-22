package main

import (
	"encoding/json"
	"fmt"
	"log"
	"runtime"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/puyokura/cmppchat/model"
)

type connectionMsg struct {
	connected bool
	host      string
}

type historyFetchMsg struct {
	host string
}

type historyLoadedMsg struct {
	messages []model.Message
}

type modelState struct {
	network   *Network
	viewport  viewport.Model
	textInput textinput.Model
	messages  []string
	err       error
	ready     bool
	loading   bool // Loading history
	// Command History
	cmdHistory []string
	historyIdx int
}

func initialModel(net *Network) modelState {
	ti := textinput.New()
	ti.Placeholder = "Type a message..."
	ti.Focus()
	ti.CharLimit = 256
	ti.Width = 20

	return modelState{
		network:    net,
		textInput:  ti,
		messages:   []string{},
		cmdHistory: []string{},
		historyIdx: -1,
	}
}

func (m modelState) Init() tea.Cmd {
	return textinput.Blink
}

func (m modelState) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	// Panic recovery to catch crashes
	defer func() {
		if r := recover(); r != nil {
			log.Printf("PANIC in Update: %v", r)
			// Try to print stack trace
			buf := make([]byte, 4096)
			n := runtime.Stack(buf, false)
			log.Printf("Stack trace:\n%s", buf[:n])
		}
	}()

	var (
		tiCmd tea.Cmd
		vpCmd tea.Cmd
	)

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyCtrlC, tea.KeyEsc:
			return m, tea.Quit
		case tea.KeyUp:
			if m.historyIdx < len(m.cmdHistory)-1 {
				m.historyIdx++
				// History is stored newest last. But usually Up means "previous command" (older).
				// Let's store history: [oldest, ..., newest]
				// Up should go: newest -> oldest
				// So index should start at len and go down?
				// Or standard shell behavior:
				// Idx = -1 (empty)
				// Up -> Idx = len-1 (newest)
				// Up -> Idx = len-2
				// Down -> Idx++

				// Let's fix the logic:
				// historyIdx points to the current history item being viewed. -1 means "current input".

				if m.historyIdx == -1 {
					// Save current input? Maybe not needed for simple version
					m.historyIdx = len(m.cmdHistory) - 1
				} else if m.historyIdx > 0 {
					m.historyIdx--
				}

				if m.historyIdx >= 0 && m.historyIdx < len(m.cmdHistory) {
					m.textInput.SetValue(m.cmdHistory[m.historyIdx])
					m.textInput.CursorEnd()
				}
			}

		case tea.KeyDown:
			if m.historyIdx != -1 {
				m.historyIdx++
				if m.historyIdx >= len(m.cmdHistory) {
					m.historyIdx = -1
					m.textInput.SetValue("")
				} else {
					m.textInput.SetValue(m.cmdHistory[m.historyIdx])
					m.textInput.CursorEnd()
				}
			}

		case tea.KeyTab:
			// Tab completion
			input := m.textInput.Value()
			if strings.HasPrefix(input, "/") {
				commands := []string{
					"/login ", "/register ", "/connect ", "/logout", "/help",
					"/admin ", "/clan ", "/kick ", "/ban ", "/unconnect",
				}

				var matches []string
				for _, cmd := range commands {
					if strings.HasPrefix(cmd, input) {
						matches = append(matches, cmd)
					}
				}

				if len(matches) == 1 {
					m.textInput.SetValue(matches[0])
					m.textInput.CursorEnd()
				} else if len(matches) > 1 {
					// Cycle? or Show list?
					// Simple cycle for now: if multiple matches, pick first that is longer than input?
					// Or just pick the first one.
					// Better: common prefix?
					// Let's just pick the first one for simplicity or cycle if we keep pressing tab?
					// Implementing cycle requires state.
					// Let's just set the first match.
					m.textInput.SetValue(matches[0])
					m.textInput.CursorEnd()
				}
			}

		case tea.KeyEnter:
			if m.textInput.Value() != "" {
				content := m.textInput.Value()
				m.textInput.SetValue("")

				// Add to history
				// Avoid duplicates at the end
				if len(m.cmdHistory) == 0 || m.cmdHistory[len(m.cmdHistory)-1] != content {
					m.cmdHistory = append(m.cmdHistory, content)
				}
				m.historyIdx = -1 // Reset history index

				// Check for client-side commands
				if strings.HasPrefix(content, "/connect ") {
					parts := strings.Fields(content)
					if len(parts) == 2 {
						host := parts[1]
						return m, func() tea.Msg {
							err := m.network.Connect(host)
							if err != nil {
								return errMsg(err)
							}
							// Start waiting for messages
							return connectionMsg{connected: true, host: host}
						}
					}
					m.messages = append(m.messages, "Usage: /connect <host>")
					m.viewport.SetContent(strings.Join(m.messages, "\n"))
					m.viewport.GotoBottom()
					return m, nil
				}

				if content == "/unconnect" {
					m.network.Disconnect()
					m.messages = append(m.messages, "Disconnected.")
					m.viewport.SetContent(strings.Join(m.messages, "\n"))
					m.viewport.GotoBottom()
					return m, nil
				}

				return m, m.network.SendMessage(content)
			}
		}

	case connectionMsg:
		if msg.connected {
			// Connected, now fetch history
			m.loading = true
			return m, func() tea.Msg {
				// We need the host used for connection.
				// For now, let's assume we can get it or pass it.
				// Since we don't store it easily, let's extract from network or pass in connectionMsg?
				// Let's modify connectionMsg to carry host.
				return historyFetchMsg{host: msg.host}
			}
		}

	case historyFetchMsg:
		return m, func() tea.Msg {
			msgs, err := m.network.FetchMessages(msg.host)
			if err != nil {
				return errMsg(err)
			}
			return historyLoadedMsg{messages: msgs}
		}

	case historyLoadedMsg:
		m.loading = false
		// Process messages
		for _, msg := range msg.messages {
			m.messages = append(m.messages, formatMessage(msg, m.viewport.Width))
		}
		m.viewport.SetContent(strings.Join(m.messages, "\n"))
		m.viewport.GotoBottom()
		return m, m.network.WaitForMessage

	case tea.WindowSizeMsg:
		headerHeight := 1
		footerHeight := 2 // Border + Input
		verticalMarginHeight := headerHeight + footerHeight

		if !m.ready {
			m.viewport = viewport.New(msg.Width, msg.Height-verticalMarginHeight)
			m.viewport.YPosition = headerHeight
			m.viewport.SetContent("")
			m.ready = true
		} else {
			m.viewport.Width = msg.Width
			m.viewport.Height = msg.Height - verticalMarginHeight
		}
		m.textInput.Width = msg.Width

	case model.Event:
		defer func() {
			if r := recover(); r != nil {
				log.Printf("Recovered panic in Event handling: %v", r)
				m.messages = append(m.messages, fmt.Sprintf("Error: %v", r))
				m.viewport.SetContent(strings.Join(m.messages, "\n"))
			}
		}()

		// Handle incoming event
		if msg.Type == model.EventMessage {
			payloadBytes, err := json.Marshal(msg.Payload)
			if err != nil {
				return m, m.network.WaitForMessage
			}

			var chatMsg model.Message
			if err := json.Unmarshal(payloadBytes, &chatMsg); err != nil {
				return m, m.network.WaitForMessage
			}

			formatted := formatMessage(chatMsg, m.viewport.Width)
			m.messages = append(m.messages, formatted)
			m.viewport.SetContent(strings.Join(m.messages, "\n"))
			m.viewport.GotoBottom()
			return m, m.network.WaitForMessage
		}
		return m, m.network.WaitForMessage

	case errMsg:
		m.err = msg
		return m, tea.Quit
	}

	m.textInput, tiCmd = m.textInput.Update(msg)
	m.viewport, vpCmd = m.viewport.Update(msg)

	return m, tea.Batch(tiCmd, vpCmd)
}

func (m modelState) View() string {
	if !m.ready {
		return "\n  Initializing..."
	}

	if m.loading {
		// Just text
		return "\n\n  Loading messages... Please wait."
	}

	return fmt.Sprintf(
		"%s\n%s\n%s",
		m.headerView(),
		m.viewport.View(),
		m.footerView(),
	)
}

func (m modelState) headerView() string {
	title := " CMPPChat "
	style := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#FFFFFF")).
		Background(lipgloss.Color("#6C5CE7")). // Nice purple
		Bold(true)

	// Actually, let's just make a full width bar
	width := m.viewport.Width
	if width == 0 {
		return ""
	}

	return style.Width(width).Render(title)
}

func (m modelState) footerView() string {
	// Styled footer with a border top
	return lipgloss.NewStyle().
		Border(lipgloss.NormalBorder(), true, false, false, false).
		BorderForeground(lipgloss.Color("#6C5CE7")).
		Width(m.viewport.Width).
		Render(m.textInput.View())
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func formatMessage(msg model.Message, width int) string {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("Panic in formatMessage: %v, msg: %+v", r, msg)
		}
	}()

	// Safety check for width
	if width < 50 {
		width = 80 // Default minimum width
	}

	// Format: │ Time  │ Sender          │ IPID            │ Message
	// Time: 5 chars (15:04)
	// Sender: 15 chars
	// IPID: 15 chars

	timeStr := msg.Timestamp.Format("15:04")

	// Sender - safely handle empty values
	rawUser := msg.Sender
	if msg.SenderDisplay != "" {
		rawUser = msg.SenderDisplay
	}
	if rawUser == "" {
		rawUser = "Unknown"
	}

	userWithColors := parseColorTags(rawUser)
	userWidth := lipgloss.Width(userWithColors)

	// Safety check
	if userWidth < 0 {
		userWidth = len(rawUser)
	}

	padding := 15 - userWidth
	if padding > 0 {
		userWithColors += strings.Repeat(" ", padding)
	}

	// IPID - safely handle empty values
	ipid := msg.SenderID
	if ipid == "" {
		ipid = "0.0.0.0"
	}
	if len(ipid) > 15 {
		ipid = ipid[:15]
	} else {
		ipid = fmt.Sprintf("%-15s", ipid)
	}

	// Colors for borders
	borderColor := lipgloss.Color("#505050")
	borderStyle := lipgloss.NewStyle().Foreground(borderColor)
	vLine := borderStyle.Render("│")

	// Prefix construction
	// │ Time  │ Sender          │ IPID            │
	prefix := fmt.Sprintf("%s %s %s %s %s %s %s ", vLine, timeStr, vLine, userWithColors, vLine, ipid, vLine)
	prefixWidth := lipgloss.Width(prefix)

	// Safety check
	if prefixWidth <= 0 || prefixWidth > width {
		prefixWidth = 50 // Fallback
	}

	// Word wrap message
	msgWidth := width - prefixWidth
	if msgWidth < 10 {
		msgWidth = 10
	}

	contentWithColors := parseColorTags(msg.Content)
	wrapped := lipgloss.NewStyle().Width(msgWidth).Render(contentWithColors)
	lines := strings.Split(wrapped, "\n")

	var result strings.Builder

	// First line
	result.WriteString(prefix)
	if len(lines) > 0 {
		result.WriteString(lines[0])
	}
	result.WriteString("\n")

	// Subsequent lines
	// │       │                 │                 │
	// We need to match the spaces of the prefix columns

	// Time column: 5 spaces
	// Sender column: userSpace (dynamic if overflow)
	userSpace := userWidth
	if userSpace < 15 {
		userSpace = 15
	}

	emptyPrefix := fmt.Sprintf("%s %s %s %s %s %s %s ",
		vLine, strings.Repeat(" ", 5),
		vLine, strings.Repeat(" ", userSpace),
		vLine, strings.Repeat(" ", 15),
		vLine)

	for i := 1; i < len(lines); i++ {
		result.WriteString(emptyPrefix)
		result.WriteString(lines[i])
		result.WriteString("\n")
	}

	return result.String()
}

// Helper to render messages with color tags
// We need to update how messages are added to the viewport.
// Currently m.messages is []string.
// We should parse the string and apply styles before adding to viewport?
// Viewport takes string content.
// So we need a function that takes "Raw <#FF0000>Text</>" and returns "Raw " + lipgloss.NewStyle().Foreground(Color("#FF0000")).Render("Text")

func parseColorTags(input string) string {
	// Simple parser for <#RRGGBB>content</>
	// This is a bit naive but should work for the clan tags.
	output := ""
	remaining := input

	for {
		start := strings.Index(remaining, "<#")
		if start == -1 {
			output += remaining
			break
		}

		output += remaining[:start]
		remaining = remaining[start:]

		// Expect <#RRGGBB>
		endTagStart := strings.Index(remaining, ">")
		if endTagStart == -1 {
			output += remaining
			break
		}

		colorCode := remaining[1:endTagStart] // #RRGGBB
		contentStart := endTagStart + 1

		remaining = remaining[contentStart:]

		endTag := strings.Index(remaining, "</>")
		if endTag == -1 {
			// Malformed, just print rest
			output += "<" + colorCode + ">" + remaining
			break
		}

		content := remaining[:endTag]
		remaining = remaining[endTag+3:]

		// Apply style
		styled := lipgloss.NewStyle().Foreground(lipgloss.Color(colorCode)).Render(content)
		output += styled
	}
	return output
}
