package panels

import (
	"github.com/charmbracelet/lipgloss"

	"file-tui/internal/config"
)

type Styles struct {
	Header         lipgloss.Style
	Footer         lipgloss.Style
	Panel          lipgloss.Style
	ActivePanel    lipgloss.Style
	FileName       lipgloss.Style
	DirName        lipgloss.Style
	Executable     lipgloss.Style
	Symlink        lipgloss.Style
	Selected       lipgloss.Style
	Marked         lipgloss.Style
	MarkedSelected lipgloss.Style
}

func NewStyles(theme *config.Theme) *Styles {
	return &Styles{
		Header: lipgloss.NewStyle().
			Background(lipgloss.Color(theme.Colors.Header)).
			Padding(0, 1),
		Footer: lipgloss.NewStyle().
			Background(lipgloss.Color(theme.Colors.Footer)).
			Padding(0, 1),
		Panel: lipgloss.NewStyle().
			Background(lipgloss.Color(theme.Colors.Panel)).
			Border(lipgloss.NormalBorder(), false, false, false, true).
			BorderForeground(lipgloss.Color("#666666")),
		ActivePanel: lipgloss.NewStyle().
			Background(lipgloss.Color(theme.Colors.ActivePanel)).
			Border(lipgloss.NormalBorder(), false, false, false, true).
			BorderForeground(lipgloss.Color("#888888")),
		FileName: lipgloss.NewStyle().
			Foreground(lipgloss.Color(theme.Colors.File)),
		DirName: lipgloss.NewStyle().
			Foreground(lipgloss.Color(theme.Colors.Dir)).
			Bold(true),
		Executable: lipgloss.NewStyle().
			Foreground(lipgloss.Color(theme.Colors.Executable)),
		Symlink: lipgloss.NewStyle().
			Foreground(lipgloss.Color(theme.Colors.Symlink)),
		Selected: lipgloss.NewStyle().
			Background(lipgloss.Color(theme.Colors.Selected)).
			Bold(true),
		Marked: lipgloss.NewStyle().
			Background(lipgloss.Color(theme.Colors.Marked)).
			Foreground(lipgloss.Color("#1a1a1a")).
			Bold(true),
		MarkedSelected: lipgloss.NewStyle().
			Background(lipgloss.Color(theme.Colors.Marked)).
			Foreground(lipgloss.Color("#1a1a1a")).
			Bold(true).
			Underline(true),
	}
}
