package panels

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"file-tui/internal/config"
	"file-tui/internal/fs"
)

type FilePanel struct {
	Path          string
	Dir           *fs.Directory
	Files         []fs.FileItem
	FilteredFiles []fs.FileItem
	Cursor        int
	Scroll        int
	Width         int
	Height        int
	IsActive      bool
	ShowHidden    bool
	SortType      fs.SortType
	Ascending     bool
	DirsFirst     bool
	Filter        string
	Styles        *Styles
}

func NewFilePanel(path string, cfg *config.Config, theme *config.Theme) *FilePanel {
	return &FilePanel{
		Path:          fs.ExpandHome(path),
		Files:         []fs.FileItem{},
		FilteredFiles: []fs.FileItem{},
		Cursor:        0,
		Scroll:        0,
		IsActive:      false,
		ShowHidden:    cfg.ShowHidden,
		SortType:      sortTypeFromString(cfg.DefaultSort),
		Ascending:     true,
		DirsFirst:     cfg.DirsFirst,
		Filter:        "",
		Styles:        NewStyles(theme),
	}
}

func sortTypeFromString(s string) fs.SortType {
	switch s {
	case "size":
		return fs.SortBySize
	case "time":
		return fs.SortByTime
	case "type":
		return fs.SortByType
	default:
		return fs.SortByName
	}
}

type LoadDirMsg struct {
	PanelIdx int
	Dir      *fs.Directory
	Err      error
}

func LoadDirectoryCmd(path string, showHidden bool, panelIdx int) tea.Cmd {
	return func() tea.Msg {
		dir, err := fs.ReadDirectory(path, showHidden)
		return LoadDirMsg{PanelIdx: panelIdx, Dir: dir, Err: err}
	}
}

func (p *FilePanel) Init() tea.Cmd {
	return LoadDirectoryCmd(p.Path, p.ShowHidden, 0)
}

func (p *FilePanel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case LoadDirMsg:
		if msg.Err == nil {
			p.Dir = msg.Dir
			p.Files = msg.Dir.Files
			p.Path = msg.Dir.Path
			p.applySorting()
			p.applyFilter()
			p.Cursor = 0
			p.Scroll = 0
		}
	}
	return p, nil
}

func (p *FilePanel) applySorting() {
	fs.SortFiles(p.Files, p.SortType, p.Ascending, p.DirsFirst)
}

func (p *FilePanel) applyFilter() {
	if p.Filter == "" {
		p.FilteredFiles = p.Files
		return
	}

	filtered := make([]fs.FileItem, 0, len(p.Files))
	for _, f := range p.Files {
		if strings.Contains(strings.ToLower(f.Name), strings.ToLower(p.Filter)) {
			filtered = append(filtered, f)
		}
	}
	p.FilteredFiles = filtered

	if p.Cursor >= len(p.FilteredFiles) {
		p.Cursor = max(0, len(p.FilteredFiles)-1)
	}
}

func (p *FilePanel) CycleSort() {
	types := []fs.SortType{fs.SortByName, fs.SortBySize, fs.SortByTime, fs.SortByType}
	found := false
	for i, t := range types {
		if t == p.SortType {
			if i == len(types)-1 {
				if !p.Ascending {
					p.SortType = types[0]
					p.Ascending = true
				} else {
					p.Ascending = false
				}
			} else if p.Ascending {
				p.Ascending = false
			} else {
				p.SortType = types[i+1]
				p.Ascending = true
			}
			found = true
			break
		}
	}
	if !found {
		p.SortType = fs.SortByName
		p.Ascending = true
	}
	p.applySorting()
	p.applyFilter()
}

func (p *FilePanel) Refresh(panelIdx int) tea.Cmd {
	return LoadDirectoryCmd(p.Path, p.ShowHidden, panelIdx)
}

func (p *FilePanel) EnterDir(panelIdx int) tea.Cmd {
	if len(p.FilteredFiles) == 0 {
		return nil
	}
	item := p.FilteredFiles[p.Cursor]
	if item.IsDir {
		p.Path = item.Path
		return LoadDirectoryCmd(item.Path, p.ShowHidden, panelIdx)
	}
	return nil
}

func (p *FilePanel) GoParent(panelIdx int) tea.Cmd {
	parent := fs.ParentDir(p.Path)
	if parent != p.Path {
		p.Path = parent
		return LoadDirectoryCmd(parent, p.ShowHidden, panelIdx)
	}
	return nil
}

func (p *FilePanel) MoveCursor(by int) {
	p.Cursor = clamp(p.Cursor+by, 0, max(0, len(p.FilteredFiles)-1))
	p.EnsureVisible()
}

func (p *FilePanel) EnsureVisible() {
	visibleHeight := p.Height - 3
	if visibleHeight <= 0 {
		return
	}
	if p.Cursor < p.Scroll {
		p.Scroll = p.Cursor
	} else if p.Cursor >= p.Scroll+visibleHeight {
		p.Scroll = p.Cursor - visibleHeight + 1
	}
}

func (p *FilePanel) ToggleMark() {
	if len(p.FilteredFiles) == 0 {
		return
	}

	item := p.FilteredFiles[p.Cursor]
	for i, f := range p.Files {
		if f.Path == item.Path {
			p.Files[i].IsMarked = !f.IsMarked
			break
		}
	}
	for i, f := range p.FilteredFiles {
		if f.Path == item.Path {
			p.FilteredFiles[i].IsMarked = !f.IsMarked
			break
		}
	}

	if p.Cursor < len(p.FilteredFiles)-1 {
		p.Cursor++
		p.EnsureVisible()
	}
}

func (p *FilePanel) MarkAll() {
	for i := range p.Files {
		p.Files[i].IsMarked = true
	}
	for i := range p.FilteredFiles {
		p.FilteredFiles[i].IsMarked = true
	}
}

func (p *FilePanel) InverseMark() {
	for i := range p.Files {
		p.Files[i].IsMarked = !p.Files[i].IsMarked
	}
	for i := range p.FilteredFiles {
		p.FilteredFiles[i].IsMarked = !p.FilteredFiles[i].IsMarked
	}
}

func (p *FilePanel) ClearMarks() {
	for i := range p.Files {
		p.Files[i].IsMarked = false
	}
	for i := range p.FilteredFiles {
		p.FilteredFiles[i].IsMarked = false
	}
}

func (p *FilePanel) GetSelectedFiles() []fs.FileItem {
	var marked []fs.FileItem
	for _, f := range p.Files {
		if f.IsMarked {
			marked = append(marked, f)
		}
	}
	if len(marked) > 0 {
		return marked
	}
	if len(p.FilteredFiles) > 0 {
		return []fs.FileItem{p.FilteredFiles[p.Cursor]}
	}
	return nil
}

func (p *FilePanel) GetCurrentItem() *fs.FileItem {
	if len(p.FilteredFiles) > 0 {
		return &p.FilteredFiles[p.Cursor]
	}
	return nil
}

func (p *FilePanel) GetMarkedStats() (count int, size int64) {
	for _, f := range p.Files {
		if f.IsMarked {
			count++
			size += f.Size
		}
	}
	return count, size
}

func (p *FilePanel) ToggleHidden(panelIdx int) tea.Cmd {
	p.ShowHidden = !p.ShowHidden
	return LoadDirectoryCmd(p.Path, p.ShowHidden, panelIdx)
}

func (p *FilePanel) SetFilter(filter string) {
	p.Filter = filter
	p.applyFilter()
}

func (p *FilePanel) View() string {
	panelStyle := p.Styles.Panel
	if p.IsActive {
		panelStyle = p.Styles.ActivePanel
	}

	header := p.renderHeader()
	body := p.renderBody()
	footer := p.renderFooter()

	content := lipgloss.JoinVertical(lipgloss.Left, header, body, footer)
	return panelStyle.Width(p.Width).Height(p.Height).Render(content)
}

func (p *FilePanel) renderHeader() string {
	pathDisplay := p.Path
	if len(pathDisplay) > p.Width-4 {
		pathDisplay = "..." + pathDisplay[len(pathDisplay)-(p.Width-7):]
	}

	style := p.Styles.Header.Width(p.Width - 2)
	if p.IsActive {
		style = style.Bold(true).Foreground(lipgloss.Color("#ffffff"))
	} else {
		style = style.Foreground(lipgloss.Color("#aaaaaa"))
	}

	return style.Render(pathDisplay)
}

func (p *FilePanel) renderBody() string {
	visibleHeight := p.Height - 3
	if visibleHeight <= 0 {
		return ""
	}

	lines := make([]string, 0, visibleHeight)
	for i := p.Scroll; i < p.Scroll+visibleHeight; i++ {
		if i >= len(p.FilteredFiles) {
			lines = append(lines, strings.Repeat(" ", p.Width-2))
			continue
		}

		item := p.FilteredFiles[i]
		isCursor := i == p.Cursor
		line := p.renderFileItem(item, isCursor)
		lines = append(lines, line)
	}

	return lipgloss.JoinVertical(lipgloss.Left, lines...)
}

func (p *FilePanel) renderFileItem(item fs.FileItem, isCursor bool) string {
	var nameStyle lipgloss.Style
	switch item.Type {
	case fs.TypeDir:
		nameStyle = p.Styles.DirName
	case fs.TypeExecutable:
		nameStyle = p.Styles.Executable
	case fs.TypeSymlink:
		nameStyle = p.Styles.Symlink
	default:
		nameStyle = p.Styles.FileName
	}

	markChar := " "
	if item.IsMarked {
		markChar = "*"
	}

	name := item.Name
	nameDisplay := name
	maxNameWidth := p.Width - 45
	if len(name) > maxNameWidth {
		nameDisplay = name[:maxNameWidth-3] + "..."
	}

	sizeStr := fs.FormatSize(item.Size)
	timeStr := item.ModTime.Format("2006-01-02 15:04")
	modeStr := fs.FormatMode(item.Mode)

	line := fmt.Sprintf("%s %-*s %10s %16s %10s",
		markChar,
		maxNameWidth,
		nameDisplay,
		sizeStr,
		timeStr,
		modeStr,
	)

	applyStyle := false
	var finalStyle lipgloss.Style
	if item.IsMarked && isCursor {
		finalStyle = p.Styles.MarkedSelected
		applyStyle = true
	} else if item.IsMarked {
		finalStyle = p.Styles.Marked
		applyStyle = true
	} else if isCursor {
		finalStyle = p.Styles.Selected
		applyStyle = true
	}

	coloredName := nameStyle.Render(nameDisplay)
	line = fmt.Sprintf("%s %s %10s %16s %10s",
		markChar,
		coloredName,
		sizeStr,
		timeStr,
		modeStr,
	)

	if len(line) > p.Width-2 {
		if p.Width-2 > 0 {
			line = line[:p.Width-2]
		} else {
			line = ""
		}
	}

	if applyStyle {
		line = finalStyle.Render(line)
	}

	return line
}

func (p *FilePanel) renderFooter() string {
	var dir *fs.Directory = p.Dir
	var (
		fileCount = 0
		dirCount  = 0
		totalSize int64
	)

	if dir != nil {
		fileCount = dir.FileCount
		dirCount = dir.DirCount
		totalSize = dir.TotalSize
	}

	markedCount, markedSize := p.GetMarkedStats()

	info := fmt.Sprintf("Dirs: %d | Files: %d | Size: %s",
		dirCount, fileCount, fs.FormatSize(totalSize))

	if markedCount > 0 {
		info += fmt.Sprintf(" | Marked: %d (%s)", markedCount, fs.FormatSize(markedSize))
	}

	if len(info) > p.Width-4 {
		if p.Width-4 > 0 {
			info = info[:p.Width-4]
		} else {
			info = ""
		}
	}

	return p.Styles.Footer.Width(p.Width - 2).Render(info)
}

func clamp(v, low, high int) int {
	if v < low {
		return low
	}
	if v > high {
		return high
	}
	return v
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
