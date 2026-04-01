// Copyright 2025 Terramate GmbH
// SPDX-License-Identifier: MPL-2.0

package ui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// Terramate color palette.
// Uses AdaptiveColor to work in both light and dark terminals.
var (
	// Primary brand colors
	colorPrimary   = lipgloss.AdaptiveColor{Light: "#7C3AED", Dark: "#A78BFA"} // Purple/Violet
	colorSecondary = lipgloss.AdaptiveColor{Light: "#059669", Dark: "#34D399"} // Green

	// Semantic colors
	colorError   = lipgloss.AdaptiveColor{Light: "#DC2626", Dark: "#F87171"} // Red
	colorWarning = lipgloss.AdaptiveColor{Light: "#D97706", Dark: "#FBBF24"} // Amber

	// Command colors colors
	colorCreate   = lipgloss.AdaptiveColor{Light: "#059669", Dark: "#34D399"} // Green
	colorReconfig = lipgloss.AdaptiveColor{Light: "#D97706", Dark: "#FBBF24"} // Amber
	colorPromote  = lipgloss.AdaptiveColor{Light: "#2563EB", Dark: "#60A5FA"} // Blueish

	// Text colors
	colorText       = lipgloss.AdaptiveColor{Light: "#1F2937", Dark: "#F9FAFB"} // Primary text
	colorTextMuted  = lipgloss.AdaptiveColor{Light: "#6B7280", Dark: "#9CA3AF"} // Muted/secondary text
	colorTextSubtle = lipgloss.AdaptiveColor{Light: "#9CA3AF", Dark: "#6B7280"} // Very subtle text

	// Background colors
	colorBgSubtle = lipgloss.AdaptiveColor{Light: "#F3F4F6", Dark: "#374151"} // Subtle background
	colorBgMuted  = lipgloss.AdaptiveColor{Light: "#E5E7EB", Dark: "#4B5563"} // Muted background

	// Border colors
	colorBorder      = lipgloss.AdaptiveColor{Light: "#D1D5DB", Dark: "#4B5563"}
	colorBorderFocus = lipgloss.AdaptiveColor{Light: "#7C3AED", Dark: "#A78BFA"} // Purple when focused

	// Scrollbar
	colorScrollTrack = lipgloss.AdaptiveColor{Light: "#E5E7EB", Dark: "#1F2937"}
	colorScrollThumb = lipgloss.AdaptiveColor{Light: "#9CA3AF", Dark: "#6B7280"}
)

func renderTerramateLogo1() string {
	logoStyle := lipgloss.NewStyle().Foreground(colorText)
	lines := []string{
		" █▙▗▖   ▗▖▟█",
		" ▄▄▐█████▌▄▄",
		" ▀▀▝▜███▛▘▀▀",
		" ▝█  ▝█▘  █▘",
	}
	return logoStyle.Render(strings.Join(lines, "\n"))
}
