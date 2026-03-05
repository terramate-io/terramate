// Copyright 2025 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package ui

import (
	"io"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textarea"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/pkg/browser"
	"github.com/terramate-io/terramate/printer"
	"github.com/terramate-io/terramate/ui/tui/cliauth"
)

const cloudSignupURL = "https://cloud.terramate.io"

type cloudLoginResultMsg struct {
	err error
}

func cloudLoginCmd(m Model) tea.Cmd {
	return func() tea.Msg {
		discard := printer.Printers{
			Stdout: printer.NewPrinter(io.Discard),
			Stderr: printer.NewPrinter(io.Discard),
		}
		err := cliauth.GoogleLogin(discard, 0, m.EngineState.CLIConfig)
		return cloudLoginResultMsg{err: err}
	}
}

func (m Model) updateCloudLogin(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.cloudLoginLoading {
		if key.Matches(msg, keys.Enter) {
			m.cloudLoginLoading = false
			m.viewState = m.nextViewAfterCloudLogin()
			return m, textarea.Blink
		}
		return m, nil
	}

	switch {
	case key.Matches(msg, keys.Left):
		if m.cloudLoginButtonIdx > 0 {
			m.cloudLoginButtonIdx--
		}
	case key.Matches(msg, keys.Right):
		if m.cloudLoginButtonIdx < 2 {
			m.cloudLoginButtonIdx++
		}
	case key.Matches(msg, keys.Enter):
		switch m.cloudLoginButtonIdx {
		case 0:
			if err := browser.OpenURL(cloudSignupURL); err != nil {
				m.cloudSignupMsg = "Could not open browser. Visit " + cloudSignupURL
			} else {
				m.cloudSignupMsg = "Opened " + cloudSignupURL + " in your browser."
			}
			return m, nil
		case 1:
			m.cloudLoginLoading = true
			m.cloudLoginButtonIdx = 0
			return m, cloudLoginCmd(m)
		case 2:
			m.viewState = m.nextViewAfterCloudLogin()
			return m, textarea.Blink
		}
	}
	return m, nil
}

func (m Model) nextViewAfterCloudLogin() ViewState {
	if len(m.EngineState.Registry.Environments) > 0 {
		return ViewEnvSelect
	}
	return ViewOverview
}

func (m Model) renderCloudLoginView() string {
	borderStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(colorBorderFocus).
		Padding(1, 2).
		Width(uiWidth)

	helpStyle := lipgloss.NewStyle().
		Foreground(colorTextMuted).
		Width(uiWidth)

	header := m.renderHeader("welcome")

	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(colorText).
		MarginBottom(1)

	descStyle := lipgloss.NewStyle().
		Foreground(colorTextMuted).
		MarginBottom(1)

	title := titleStyle.Render("Welcome to Terramate! 👋")
	desc := descStyle.Render("Connect to Terramate Cloud for the full experience,\nor skip to get started locally.")

	logo := renderTerramateLogo1()
	textBlock := lipgloss.JoinVertical(lipgloss.Left, title, desc)
	logoAndText := lipgloss.JoinHorizontal(lipgloss.Top, logo, "   ", textBlock)

	var body string
	if m.cloudLoginLoading {
		msgStyle := lipgloss.NewStyle().
			Foreground(colorPrimary).
			Bold(true)

		body = msgStyle.Render("Waiting for authentication — please continue in the browser.") + "\n\n" +
			m.renderCloudLoginButtons()
	} else {
		body = m.renderCloudLoginButtons()
	}

	panelContent := lipgloss.JoinVertical(lipgloss.Left, logoAndText, body)
	panel := borderStyle.Render(panelContent)
	help := helpStyle.Render(m.finalHelpText(""))

	content := lipgloss.JoinVertical(lipgloss.Left, header, panel, help)
	return lipgloss.NewStyle().Padding(1, 2).Render(content)
}

func (m Model) renderCloudLoginButtons() string {
	activeStyle := lipgloss.NewStyle().
		Padding(0, 1).
		Background(colorPrimary).
		Foreground(lipgloss.Color("#000000")).
		Bold(true)

	buttonStyle := lipgloss.NewStyle().
		Padding(0, 1).
		Background(colorBgSubtle).
		Foreground(colorText)

	var loginBtn, signupBtn, skipBtn string

	if m.cloudLoginLoading {
		if m.cloudLoginButtonIdx == 0 {
			skipBtn = activeStyle.Render("Skip")
		} else {
			skipBtn = buttonStyle.Render("Skip")
		}
		return skipBtn
	}

	renderBtn := func(label string, idx int) string {
		if m.cloudLoginButtonIdx == idx {
			return activeStyle.Render(label)
		}
		return buttonStyle.Render(label)
	}

	signupBtn = renderBtn("Sign up", 0)
	loginBtn = renderBtn("Login", 1)
	skipBtn = renderBtn("Skip", 2)

	buttons := lipgloss.JoinHorizontal(lipgloss.Top, signupBtn, " ", loginBtn, " ", skipBtn)

	if m.cloudSignupMsg != "" {
		msgStyle := lipgloss.NewStyle().
			Foreground(colorTextMuted).
			Italic(true)
		return lipgloss.JoinHorizontal(lipgloss.Top, buttons, "  ", msgStyle.Render(m.cloudSignupMsg))
	}

	return buttons
}
