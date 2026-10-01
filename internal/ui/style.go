package ui

import "github.com/charmbracelet/lipgloss"

// Catppuccin Latte / Mocha. Dark terminals pick the Mocha side.
var (
	cText      = lipgloss.AdaptiveColor{Light: "#4c4f69", Dark: "#cdd6f4"}
	cMuted     = lipgloss.AdaptiveColor{Light: "#6c6f85", Dark: "#a6adc8"}
	cAccent    = lipgloss.AdaptiveColor{Light: "#1e66f5", Dark: "#89b4fa"}
	cPink      = lipgloss.AdaptiveColor{Light: "#ea76cb", Dark: "#f5c2e7"}
	cGreen     = lipgloss.AdaptiveColor{Light: "#40a02b", Dark: "#a6e3a1"}
	cYellow    = lipgloss.AdaptiveColor{Light: "#df8e1d", Dark: "#f9e2af"}
	cRed       = lipgloss.AdaptiveColor{Light: "#d20f39", Dark: "#f38ba8"}
	cSurface   = lipgloss.AdaptiveColor{Light: "#e6e9ef", Dark: "#313244"}
	cCrust     = lipgloss.AdaptiveColor{Light: "#dce0e8", Dark: "#11111b"}
	cBorder    = lipgloss.AdaptiveColor{Light: "#bcc0cc", Dark: "#45475a"}
	cBadgeText = lipgloss.AdaptiveColor{Light: "#eff1f5", Dark: "#11111b"}
)

func styleKey() lipgloss.Style {
	return lipgloss.NewStyle().Bold(true).Foreground(cAccent)
}

func styleLabel() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(cMuted)
}

func styleText() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(cText)
}
