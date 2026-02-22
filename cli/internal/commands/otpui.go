package commands

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"mycli.sh/cli/internal/auth"
	"mycli.sh/cli/internal/client"
	"mycli.sh/cli/internal/termui"
)

// ANSI color helpers for OTP view
func ansiColor(r, g, b int, s string) string {
	return fmt.Sprintf("\033[38;2;%d;%d;%dm%s\033[0m", r, g, b, s)
}

var (
	colorViolet = func(s string) string { return ansiColor(139, 92, 246, s) }
	colorGray   = func(s string) string { return ansiColor(63, 63, 70, s) }
	colorDim    = func(s string) string { return ansiColor(113, 113, 122, s) }
	colorMuted  = func(s string) string { return ansiColor(161, 161, 170, s) }
	colorYellow = func(s string) string { return ansiColor(250, 204, 21, s) }
	colorGreen  = func(s string) string { return ansiColor(74, 222, 128, s) }
)

// Message types

type tickMsg time.Time

type pollResultMsg struct {
	token *auth.TokenResponse
	err   error
}

type verifyResultMsg struct {
	err error
}

type resendResultMsg struct {
	expiresIn int
	err       error
}

// Commands (async operations)

func pollCmd(c *client.Client, deviceCode string, interval time.Duration) tea.Cmd {
	return func() tea.Msg {
		time.Sleep(interval)
		token, err := c.PollDeviceToken(deviceCode)
		if err != nil {
			if apiErr, ok := err.(*client.APIError); ok && apiErr.Code == "AUTHORIZATION_PENDING" {
				// Not yet authorized — schedule another poll
				return pollCmd(c, deviceCode, interval)()
			}
			return pollResultMsg{err: err}
		}
		return pollResultMsg{token: token}
	}
}

func verifyOTPCmd(c *client.Client, deviceCode, code string) tea.Cmd {
	return func() tea.Msg {
		err := c.VerifyOTP(deviceCode, code)
		return verifyResultMsg{err: err}
	}
}

func resendCmd(c *client.Client, deviceCode, email string) tea.Cmd {
	return func() tea.Msg {
		expiresIn, err := c.ResendVerification(deviceCode, email)
		return resendResultMsg{expiresIn: expiresIn, err: err}
	}
}

func tickCmd() tea.Cmd {
	return tea.Tick(1*time.Second, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

// Model

type otpModel struct {
	// Input
	code string // accumulated digits (max 6)

	// Timer
	deadline  time.Time
	remaining time.Duration

	// Resend
	showResendHint      bool
	resendHintAfter     time.Time
	resendCooldownUntil time.Time

	// Status feedback
	status    string // "", "verifying", "resending", "verified", "error", "cooldown"
	statusMsg string // feedback text shown to user

	// Result (read after program exits)
	token *auth.TokenResponse
	err   error
	done  bool

	// Dependencies (injected)
	client     *client.Client
	deviceCode string
	email      string
	interval   time.Duration
}

func newOTPModel(c *client.Client, deviceCode, email string, interval, expiresIn time.Duration) otpModel {
	now := time.Now()
	return otpModel{
		deadline:            now.Add(expiresIn),
		remaining:           expiresIn,
		resendHintAfter:     now.Add(30 * time.Second),
		resendCooldownUntil: now.Add(30 * time.Second),
		client:              c,
		deviceCode:          deviceCode,
		email:               email,
		interval:            interval,
	}
}

func (m otpModel) Init() tea.Cmd {
	return tea.Batch(
		tickCmd(),
		pollCmd(m.client, m.deviceCode, m.interval),
	)
}

func (m otpModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyCtrlC, tea.KeyEsc:
			m.err = fmt.Errorf("login cancelled")
			m.done = true
			return m, tea.Quit

		case tea.KeyBackspace:
			if len(m.code) > 0 && m.status != "verifying" {
				m.code = m.code[:len(m.code)-1]
			}
			return m, nil

		case tea.KeyEnter:
			if len(m.code) == 6 && m.status != "verifying" {
				m.status = "verifying"
				m.statusMsg = "Verifying code..."
				return m, verifyOTPCmd(m.client, m.deviceCode, m.code)
			}
			return m, nil

		case tea.KeyRunes:
			for _, r := range msg.Runes {
				// Handle 'r'/'R' to resend when code is empty
				if (r == 'r' || r == 'R') && len(m.code) == 0 && m.status != "resending" && m.status != "verifying" {
					if time.Now().Before(m.resendCooldownUntil) {
						m.status = "cooldown"
						m.statusMsg = ""
						return m, nil
					}
					m.status = "resending"
					m.statusMsg = "Resending verification email..."
					return m, resendCmd(m.client, m.deviceCode, m.email)
				}
				// Accept digits
				if r >= '0' && r <= '9' && len(m.code) < 6 && m.status != "verifying" {
					m.code += string(r)
					// Clear error status when user starts typing again
					if m.status == "error" {
						m.status = ""
						m.statusMsg = ""
					}
					// Auto-submit when 6 digits entered
					if len(m.code) == 6 {
						m.status = "verifying"
						m.statusMsg = "Verifying code..."
						return m, verifyOTPCmd(m.client, m.deviceCode, m.code)
					}
				}
			}
			return m, nil
		}

	case tickMsg:
		m.remaining = time.Until(m.deadline)
		if m.remaining <= 0 {
			m.err = fmt.Errorf("login timed out — verification expired")
			m.done = true
			return m, tea.Quit
		}
		if !m.showResendHint && time.Now().After(m.resendHintAfter) {
			m.showResendHint = true
		}
		if m.status == "cooldown" && !time.Now().Before(m.resendCooldownUntil) {
			m.status = ""
			m.statusMsg = ""
		}
		return m, tickCmd()

	case pollResultMsg:
		if msg.err != nil {
			if apiErr, ok := msg.err.(*client.APIError); ok && apiErr.Code == "EXPIRED_TOKEN" {
				m.err = fmt.Errorf("login timed out — verification expired")
			} else {
				m.err = fmt.Errorf("login failed: %w", msg.err)
			}
			m.done = true
			return m, tea.Quit
		}
		// Login completed via magic link or poll picked up OTP verification
		m.token = msg.token
		m.done = true
		return m, tea.Quit

	case verifyResultMsg:
		if msg.err != nil {
			if apiErr, ok := msg.err.(*client.APIError); ok && apiErr.Code == "TOO_MANY_ATTEMPTS" {
				m.statusMsg = "Too many attempts — type R to resend"
			} else {
				m.statusMsg = "Invalid code, try again"
			}
			m.status = "error"
			m.code = ""
			return m, nil
		}
		m.status = "verified"
		m.statusMsg = "Verified! Completing login..."
		return m, nil // Poll will pick up the authorized session

	case resendResultMsg:
		if msg.err != nil {
			m.status = "error"
			m.statusMsg = "Failed to resend"
		} else {
			m.status = ""
			m.statusMsg = termui.Green("Sent!")
			m.resendCooldownUntil = time.Now().Add(30 * time.Second)
			m.resendHintAfter = time.Now().Add(30 * time.Second)
			m.showResendHint = false
			if msg.expiresIn > 0 {
				m.deadline = time.Now().Add(time.Duration(msg.expiresIn) * time.Second)
			}
		}
		return m, nil
	}

	return m, nil
}

func (m otpModel) View() string {
	var b strings.Builder

	// Logo
	logo := termui.Violet(">") + " " + termui.Bold("my") + termui.Violet("cli")
	b.WriteString("  " + logo + "\n\n")

	// Title + subtitle
	b.WriteString("  " + termui.Bold("Verify your email") + "\n")
	b.WriteString("  " + colorMuted("We sent a code to "+m.email) + "\n\n")

	// Digit boxes — build 3 lines (top, middle, bottom) manually
	var tops, mids, bots []string
	for i := 0; i < 6; i++ {
		var color func(string) string
		var ch string
		if i < len(m.code) {
			color = colorViolet
			ch = string(m.code[i])
		} else if i == len(m.code) {
			color = func(s string) string { return s } // default/white
			ch = "_"
		} else {
			color = colorGray
			ch = " "
		}
		tops = append(tops, color("╭───╮"))
		mids = append(mids, color("│")+" "+color(ch)+" "+color("│"))
		bots = append(bots, color("╰───╯"))
	}
	b.WriteString("  " + strings.Join(tops, " ") + "\n")
	b.WriteString("  " + strings.Join(mids, " ") + "\n")
	b.WriteString("  " + strings.Join(bots, " ") + "\n\n")

	// Timer + resend hint
	mins := int(m.remaining.Minutes())
	secs := int(m.remaining.Seconds()) % 60
	timer := fmt.Sprintf("%d:%02d remaining", mins, secs)
	if m.showResendHint {
		if time.Now().Before(m.resendCooldownUntil) {
			remaining := int(time.Until(m.resendCooldownUntil).Seconds()) + 1
			timer += fmt.Sprintf(" · Resend in %ds", remaining)
		} else {
			timer += " · Type R to resend"
		}
	}
	b.WriteString("  " + colorDim(timer))

	// Status feedback
	switch m.status {
	case "cooldown":
		remaining := int(time.Until(m.resendCooldownUntil).Seconds()) + 1
		b.WriteString("\n  " + colorYellow(fmt.Sprintf("Wait %ds before resending", remaining)))
	case "verifying", "resending":
		b.WriteString("\n  " + colorDim(m.statusMsg))
	case "verified":
		b.WriteString("\n  " + colorGreen(m.statusMsg))
	case "error":
		b.WriteString("\n  " + colorYellow(m.statusMsg))
	default:
		if m.statusMsg != "" {
			b.WriteString("\n  " + m.statusMsg)
		}
	}

	b.WriteString("\n")
	return b.String()
}

// Public entry point

func runOTPVerification(c *client.Client, deviceCode, email string, interval, expiresIn time.Duration) (*auth.TokenResponse, error) {
	model := newOTPModel(c, deviceCode, email, interval, expiresIn)
	p := tea.NewProgram(model)
	finalModel, err := p.Run()
	if err != nil {
		return nil, fmt.Errorf("TUI error: %w", err)
	}

	m := finalModel.(otpModel)
	if m.err != nil {
		return nil, m.err
	}
	return m.token, nil
}
