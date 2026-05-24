package cli

import "github.com/charmbracelet/lipgloss"

type tuiTheme struct{}

func (t tuiTheme) title() lipgloss.Style {
	return lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#c4b5fd"))
}

func (t tuiTheme) muted() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(lipgloss.Color("#71717a"))
}

func (t tuiTheme) selected() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(lipgloss.Color("#f5f3ff")).Background(lipgloss.Color("#3b2f63")).Bold(true)
}

func (t tuiTheme) panel() lipgloss.Style {
	return lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("#3f3f46")).Padding(0, 1)
}

func (t tuiTheme) danger() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(lipgloss.Color("#f87171")).Bold(true)
}

func (t tuiTheme) ok() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(lipgloss.Color("#86efac")).Bold(true)
}

func (t tuiTheme) warning() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(lipgloss.Color("#fbbf24")).Bold(true)
}

func (t tuiTheme) running() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(lipgloss.Color("#67e8f9")).Bold(true)
}

func (t tuiTheme) chrome() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(lipgloss.Color("#d4d4d8")).Background(lipgloss.Color("#18181b"))
}

func (t tuiTheme) activeTab() lipgloss.Style {
	border := lipgloss.Border{
		Top: "─", Bottom: " ", Left: "│", Right: "│",
		TopLeft: "╭", TopRight: "╮", BottomLeft: "┘", BottomRight: "└",
	}
	return lipgloss.NewStyle().
		Border(border).
		BorderForeground(lipgloss.Color("#8b5cf6")).
		Foreground(lipgloss.Color("#f5f3ff")).
		Bold(true).
		Padding(0, 1)
}

func (t tuiTheme) inactiveTab() lipgloss.Style {
	border := lipgloss.Border{
		Top: "─", Bottom: "─", Left: "│", Right: "│",
		TopLeft: "╭", TopRight: "╮", BottomLeft: "┴", BottomRight: "┴",
	}
	return lipgloss.NewStyle().
		Border(border).
		BorderForeground(lipgloss.Color("#8b5cf6")).
		Foreground(lipgloss.Color("#a1a1aa")).
		Padding(0, 1)
}

func (t tuiTheme) tabRule() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(lipgloss.Color("#8b5cf6"))
}

func (t tuiTheme) statusKey() lipgloss.Style {
	return lipgloss.NewStyle().
		Background(lipgloss.Color("#ec4899")).
		Foreground(lipgloss.Color("#ffffff")).
		Bold(true).
		Padding(0, 1)
}

func (t tuiTheme) statusValue() lipgloss.Style {
	return lipgloss.NewStyle().
		Background(lipgloss.Color("#27272a")).
		Foreground(lipgloss.Color("#d4d4d8")).
		Padding(0, 1)
}
