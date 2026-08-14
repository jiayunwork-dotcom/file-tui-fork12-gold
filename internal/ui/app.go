package ui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"file-tui/internal/config"
	"file-tui/internal/extra"
	fileops "file-tui/internal/fileops"
	fsutil "file-tui/internal/fs"
	"file-tui/internal/navigation"
	"file-tui/internal/preview"
	"file-tui/internal/ui/panels"
)

type Mode int

const (
	ModeNormal Mode = iota
	ModeInputPath
	ModeSearch
	ModeShell
	ModeChmod
	ModeConflict
	ModeConfirm
	ModeProgress
	ModePreview
	ModeBookmarks
	ModeDiskUsage
	ModeFilter
	ModeRename
	ModeNewFile
	ModeNewDir
)

type ConfirmType int

const (
	ConfirmDeleteSingle ConfirmType = iota
	ConfirmDeleteBatch
)

type App struct {
	Panels      [2]*panels.FilePanel
	ActivePanel int
	Width       int
	Height      int
	Cfg         *config.Config
	Theme       *config.Theme
	Styles      *panels.Styles
	Mode        Mode

	Preview   *preview.Preview
	Bookmarks *navigation.Bookmarks

	Input       string
	InputLabel  string
	InputCursor int

	SearchQuery     string
	SearchRecursive bool
	SearchResults   []navigation.SearchResult
	SearchCursor    int

	ShellOutput string

	ChmodPath  string
	ChmodValue string

	ConflictFile      string
	ConflictSrc       string
	ConflictDst       string
	ConflictType      string
	ConflictReqChan   <-chan fileops.ConflictRequestMsg
	ConflictRespChan  chan int
	SrcInfo           *fsutil.FileItem
	DstInfo           *fsutil.FileItem

	ConfirmType ConfirmType
	ConfirmMsg  string

	ProgressMsg    fileops.ProgressMsg
	ProgressCancel chan struct{}
	ProgressChan   chan fileops.ProgressMsg

	DiskItems  []extra.DiskItem
	DiskCursor int

	FilterInput string

	RenameOldPath string
	RenameOldName string

	ErrMsg string
}

func NewApp(cfg *config.Config) *App {
	home, err := os.UserHomeDir()
	if err != nil {
		home = "."
	}

	theme := cfg.GetTheme()

	app := &App{
		Panels: [2]*panels.FilePanel{
			panels.NewFilePanel(".", cfg, theme),
			panels.NewFilePanel(home, cfg, theme),
		},
		ActivePanel: 0,
		Cfg:         cfg,
		Theme:       theme,
		Styles:      panels.NewStyles(theme),
		Mode:        ModeNormal,
		Preview:     preview.NewPreview(),
		Bookmarks:   navigation.NewBookmarks(cfg.Bookmarks),
		ErrMsg:      "",
	}

	app.Panels[0].IsActive = true

	return app
}

func (a *App) Init() tea.Cmd {
	return tea.Batch(
		panels.LoadDirectoryCmd(a.Panels[0].Path, a.Panels[0].ShowHidden, 0),
		panels.LoadDirectoryCmd(a.Panels[1].Path, a.Panels[1].ShowHidden, 1),
	)
}

func (a *App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		a.Width = msg.Width
		a.Height = msg.Height
		a.layout()

	case panels.LoadDirMsg:
		if msg.PanelIdx >= 0 && msg.PanelIdx < 2 {
			panel := a.Panels[msg.PanelIdx]
			if msg.Err == nil {
				panel.Dir = msg.Dir
				panel.Files = msg.Dir.Files
				panel.Path = msg.Dir.Path
				fsutil.SortFiles(panel.Files, panel.SortType, panel.Ascending, panel.DirsFirst)
				panel.SetFilter(panel.Filter)
				panel.Cursor = 0
				panel.Scroll = 0
			}
		}

	case fileops.ProgressMsg:
		a.ProgressMsg = msg
		if msg.Done || msg.Error != nil {
			if a.ProgressCancel != nil {
				close(a.ProgressCancel)
				a.ProgressCancel = nil
			}
			if msg.Error != nil {
				a.ErrMsg = msg.Error.Error()
			}
			if a.ConflictRespChan != nil {
				close(a.ConflictRespChan)
				a.ConflictRespChan = nil
				a.ConflictReqChan = nil
			}
			a.Mode = ModeNormal
			idx1 := a.ActivePanel
			idx2 := 1 - a.ActivePanel
			return a, tea.Batch(
				a.Panels[idx1].Refresh(idx1),
				a.Panels[idx2].Refresh(idx2),
			)
		}
		return a, a.nextProgressMsg()

	case fileops.ConflictRequestMsg:
		return a, a.handleConflictRequest(msg)

	case tea.KeyMsg:
		action := a.handleKey(msg.String())
		if action != nil {
			return a, action
		}
	}

	return a, nil
}

func (a *App) handleConflictRequest(msg fileops.ConflictRequestMsg) tea.Cmd {
	srcInfo, _ := os.Stat(msg.Src)
	dstInfo, _ := os.Stat(msg.Dst)

	a.ConflictSrc = msg.Src
	a.ConflictDst = msg.Dst
	a.ConflictFile = filepath.Base(msg.Src)
	a.Mode = ModeConflict

	if srcInfo != nil {
		a.SrcInfo = &fsutil.FileItem{
			Name:    filepath.Base(msg.Src),
			Path:    msg.Src,
			Size:    srcInfo.Size(),
			ModTime: srcInfo.ModTime(),
			IsDir:   srcInfo.IsDir(),
		}
	} else {
		a.SrcInfo = nil
	}

	if dstInfo != nil {
		a.DstInfo = &fsutil.FileItem{
			Name:    filepath.Base(msg.Dst),
			Path:    msg.Dst,
			Size:    dstInfo.Size(),
			ModTime: dstInfo.ModTime(),
			IsDir:   dstInfo.IsDir(),
		}
	} else {
		a.DstInfo = nil
	}

	return nil
}

func (a *App) listenForConflicts() tea.Cmd {
	if a.ConflictReqChan == nil {
		return nil
	}
	return func() tea.Msg {
		msg, ok := <-a.ConflictReqChan
		if !ok {
			return nil
		}
		return msg
	}
}

func (a *App) handleKey(key string) tea.Cmd {
	if a.ErrMsg != "" && key != "esc" {
		a.ErrMsg = ""
	}

	switch a.Mode {
	case ModeNormal:
		return a.handleNormalMode(key)
	case ModePreview:
		return a.handlePreviewMode(key)
	case ModeInputPath, ModeSearch, ModeShell, ModeChmod, ModeFilter, ModeRename, ModeNewFile, ModeNewDir:
		return a.handleInputMode(key)
	case ModeConflict:
		return a.handleConflictMode(key)
	case ModeConfirm:
		return a.handleConfirmMode(key)
	case ModeProgress:
		if key == "esc" {
			if a.ProgressCancel != nil {
				select {
				case a.ProgressCancel <- struct{}{}:
				default:
				}
			}
		}
	case ModeBookmarks:
		return a.handleBookmarksMode(key)
	case ModeDiskUsage:
		return a.handleDiskUsageMode(key)
	}
	return nil
}

func (a *App) handleNormalMode(key string) tea.Cmd {
	k := a.Cfg.Keys
	panel := a.Panels[a.ActivePanel]
	_ = 1 - a.ActivePanel

	switch key {
	case k.Up, "k":
		panel.MoveCursor(-1)
	case k.Down, "j":
		panel.MoveCursor(1)
	case k.TabSwitch:
		a.switchPanel()
	case k.Enter:
		cmd := panel.EnterDir(a.ActivePanel)
		if cmd != nil {
			return cmd
		}
	case k.Backspace:
		return panel.GoParent(a.ActivePanel)
	case k.Mark:
		panel.ToggleMark()
	case k.MarkAll:
		panel.MarkAll()
	case k.InverseMark:
		panel.InverseMark()
	case k.Copy:
		return a.startCopyMove(false)
	case k.Move:
		return a.startCopyMove(true)
	case k.Delete:
		return a.startDelete()
	case k.Preview:
		return a.openPreview()
	case k.Bookmarks:
		a.Mode = ModeBookmarks
	case k.PathInput:
		a.Mode = ModeInputPath
		a.Input = ""
		a.InputLabel = "Go to path: "
	case k.Search:
		a.Mode = ModeSearch
		a.Input = ""
		a.InputLabel = "Search (Ctrl+R toggle recursive): "
		a.SearchQuery = ""
		a.SearchRecursive = false
		a.SearchResults = nil
		a.SearchCursor = 0
	case k.Sort:
		panel.CycleSort()
	case k.ShowHidden:
		return panel.ToggleHidden(a.ActivePanel)
	case k.Shell:
		a.Mode = ModeShell
		a.Input = ""
		a.InputLabel = "Shell command: "
		a.ShellOutput = ""
	case k.Chmod:
		item := panel.GetCurrentItem()
		if item != nil {
			a.Mode = ModeChmod
			a.ChmodPath = item.Path
			a.Input = ""
			a.InputLabel = "Permissions (755 or rwxr-xr-x): "
			a.ChmodValue = ""
		}
	case k.DiskUsage:
		return a.startDiskUsage()
	case "f2":
		return a.startRename()
	case "N":
		a.Mode = ModeNewFile
		a.Input = ""
		a.InputLabel = "New file name: "
		a.InputCursor = 0
	case "D":
		a.Mode = ModeNewDir
		a.Input = ""
		a.InputLabel = "New directory name: "
		a.InputCursor = 0
	case "ctrl+l":
		a.Panels[a.ActivePanel].SetFilter("")
		a.FilterInput = ""
	case "q", "ctrl+c":
		return tea.Quit
	}

	return nil
}

func (a *App) switchPanel() {
	a.Panels[a.ActivePanel].IsActive = false
	a.ActivePanel = 1 - a.ActivePanel
	a.Panels[a.ActivePanel].IsActive = true
}

func (a *App) handleInputMode(key string) tea.Cmd {
	switch key {
	case "esc":
		a.Mode = ModeNormal
		a.Input = ""
	case "enter":
		return a.executeInput()
	case "backspace":
		if len(a.Input) > 0 {
			a.Input = a.Input[:len(a.Input)-1]
		}
	case "tab":
		if a.Mode == ModeInputPath {
			completions := navigation.CompletePath(a.Input)
			if len(completions) == 1 {
				a.Input = completions[0]
			}
		}
	case "ctrl+r":
		if a.Mode == ModeSearch {
			a.SearchRecursive = !a.SearchRecursive
		}
	default:
		if len(key) == 1 {
			a.Input += key
		}
	}
	return nil
}

func (a *App) executeInput() tea.Cmd {
	panel := a.Panels[a.ActivePanel]

	switch a.Mode {
	case ModeInputPath:
		if a.Input == "" {
			a.Mode = ModeNormal
			return nil
		}
		path := fsutil.ExpandHome(a.Input)
		if !fsutil.IsDir(path) {
			a.ErrMsg = "Not a directory: " + path
			a.Mode = ModeNormal
			return nil
		}
		a.Mode = ModeNormal
		panel.Path = path
		a.Input = ""
		return panels.LoadDirectoryCmd(path, panel.ShowHidden, a.ActivePanel)

	case ModeSearch:
		if a.Input == "" {
			a.Mode = ModeNormal
			return nil
		}
		results := navigation.SearchFiles(panel.Path, a.Input, a.SearchRecursive)
		if len(results) == 0 {
			a.ErrMsg = "No matches found"
			a.Mode = ModeNormal
			return nil
		}
		first := results[0]
		if first.IsDir {
			panel.Path = first.Path
			a.Mode = ModeNormal
			a.Input = ""
			return panels.LoadDirectoryCmd(first.Path, panel.ShowHidden, a.ActivePanel)
		} else {
			a.ErrMsg = "Found: " + first.Path
			a.Mode = ModeNormal
			return nil
		}

	case ModeShell:
		if a.Input == "" {
			a.Mode = ModeNormal
			return nil
		}
		output, err := extra.RunCommand(panel.Path, a.Input)
		if err != nil {
			output += "\nError: " + err.Error()
		}
		a.ShellOutput = output
		a.Input = ""

	case ModeChmod:
		if a.Input == "" {
			a.Mode = ModeNormal
			return nil
		}
		mode, err := extra.ParsePermissions(a.Input)
		if err != nil {
			a.ErrMsg = "Invalid permissions: " + err.Error()
			a.Mode = ModeNormal
			return nil
		}
		if err := extra.Chmod(a.ChmodPath, mode); err != nil {
			a.ErrMsg = "Chmod failed: " + err.Error()
		}
		a.Mode = ModeNormal
		a.Input = ""
		return panel.Refresh(a.ActivePanel)

	case ModeFilter:
		panel.SetFilter(a.Input)
		a.FilterInput = a.Input
		a.Mode = ModeNormal
		a.Input = ""

	case ModeRename:
		if a.Input == "" {
			a.Mode = ModeNormal
			a.Input = ""
			return nil
		}
		return a.executeRename()

	case ModeNewFile:
		if a.Input == "" {
			a.Mode = ModeNormal
			a.Input = ""
			return nil
		}
		return a.executeNewFile()

	case ModeNewDir:
		if a.Input == "" {
			a.Mode = ModeNormal
			a.Input = ""
			return nil
		}
		return a.executeNewDir()
	}

	return nil
}

func (a *App) startRename() tea.Cmd {
	panel := a.Panels[a.ActivePanel]
	item := panel.GetCurrentItem()
	if item == nil {
		return nil
	}

	a.Mode = ModeRename
	a.RenameOldPath = item.Path
	a.RenameOldName = item.Name

	ext := filepath.Ext(item.Name)
	baseName := strings.TrimSuffix(item.Name, ext)
	a.Input = baseName
	a.InputCursor = len(baseName)
	a.InputLabel = "Rename: " + ext

	return nil
}

func (a *App) executeRename() tea.Cmd {
	panel := a.Panels[a.ActivePanel]
	ext := filepath.Ext(a.RenameOldName)
	newName := a.Input + ext

	if newName == a.RenameOldName {
		a.Mode = ModeNormal
		a.Input = ""
		return nil
	}

	newPath := filepath.Join(filepath.Dir(a.RenameOldPath), newName)
	if fsutil.Exists(newPath) {
		a.ErrMsg = "File already exists: " + newName
		a.Mode = ModeNormal
		a.Input = ""
		return nil
	}

	if err := os.Rename(a.RenameOldPath, newPath); err != nil {
		a.ErrMsg = "Rename failed: " + err.Error()
		a.Mode = ModeNormal
		a.Input = ""
		return nil
	}

	a.Mode = ModeNormal
	a.Input = ""
	return panel.Refresh(a.ActivePanel)
}

func (a *App) executeNewFile() tea.Cmd {
	panel := a.Panels[a.ActivePanel]
	newPath := filepath.Join(panel.Path, a.Input)

	if fsutil.Exists(newPath) {
		a.ErrMsg = "File already exists: " + a.Input
		a.Mode = ModeNormal
		a.Input = ""
		return nil
	}

	file, err := os.Create(newPath)
	if err != nil {
		a.ErrMsg = "Create failed: " + err.Error()
		a.Mode = ModeNormal
		a.Input = ""
		return nil
	}
	file.Close()

	a.Mode = ModeNormal
	a.Input = ""

	return a.refreshAndSelect(panel, newPath)
}

func (a *App) executeNewDir() tea.Cmd {
	panel := a.Panels[a.ActivePanel]
	newPath := filepath.Join(panel.Path, a.Input)

	if fsutil.Exists(newPath) {
		a.ErrMsg = "Directory already exists: " + a.Input
		a.Mode = ModeNormal
		a.Input = ""
		return nil
	}

	if err := os.MkdirAll(newPath, 0755); err != nil {
		a.ErrMsg = "Create directory failed: " + err.Error()
		a.Mode = ModeNormal
		a.Input = ""
		return nil
	}

	a.Mode = ModeNormal
	a.Input = ""

	return a.refreshAndSelect(panel, newPath)
}

func (a *App) refreshAndSelect(panel *panels.FilePanel, targetPath string) tea.Cmd {
	return func() tea.Msg {
		dir, err := fsutil.ReadDirectory(panel.Path, panel.ShowHidden)
		if err != nil {
			return panels.LoadDirMsg{PanelIdx: a.ActivePanel, Dir: dir, Err: err}
		}

		fsutil.SortFiles(dir.Files, panel.SortType, panel.Ascending, panel.DirsFirst)

		for i, f := range dir.Files {
			if f.Path == targetPath {
				panel.Cursor = i
				break
			}
		}

		return panels.LoadDirMsg{PanelIdx: a.ActivePanel, Dir: dir, Err: nil}
	}
}

func (a *App) startCopyMove(isMove bool) tea.Cmd {
	srcPanel := a.Panels[a.ActivePanel]
	dstPanel := a.Panels[1-a.ActivePanel]

	files := srcPanel.GetSelectedFiles()
	if len(files) == 0 {
		a.ErrMsg = "No files selected"
		return nil
	}

	a.Mode = ModeProgress
	a.ProgressCancel = make(chan struct{}, 1)
	a.ProgressChan = make(chan fileops.ProgressMsg, 100)
	a.ProgressMsg = fileops.ProgressMsg{
		Current: 0,
		Total:   len(files),
	}

	conflictReqChan := make(chan fileops.ConflictRequestMsg, 1)
	conflictRespChan := make(chan int, 1)
	a.ConflictReqChan = conflictReqChan
	a.ConflictRespChan = conflictRespChan

	opts := &fileops.Options{
		ConflictRequest:  conflictReqChan,
		ConflictResponse: conflictRespChan,
	}

	go func() {
		var err error
		if isMove {
			err = fileops.MoveDirectory(srcPanel.Path, dstPanel.Path, files, opts, a.ProgressChan, a.ProgressCancel)
		} else {
			err = fileops.CopyDirectory(srcPanel.Path, dstPanel.Path, files, opts, a.ProgressChan, a.ProgressCancel)
		}
		if err != nil {
			a.ProgressChan <- fileops.ProgressMsg{Error: err}
		}
		close(conflictReqChan)
		close(a.ProgressChan)
	}()

	return tea.Batch(a.nextProgressMsg(), a.listenForConflicts())
}

func (a *App) nextProgressMsg() tea.Cmd {
	return func() tea.Msg {
		msg, ok := <-a.ProgressChan
		if !ok {
			return fileops.ProgressMsg{Done: true}
		}
		return msg
	}
}

func (a *App) startDelete() tea.Cmd {
	panel := a.Panels[a.ActivePanel]
	files := panel.GetSelectedFiles()
	if len(files) == 0 {
		a.ErrMsg = "No files selected"
		return nil
	}

	if len(files) == 1 {
		f := files[0]
		a.ConfirmType = ConfirmDeleteSingle
		a.ConfirmMsg = fmt.Sprintf("Delete %s (%s)? [y/N]", f.Name, fsutil.FormatSize(f.Size))
	} else {
		var totalSize int64
		for _, f := range files {
			totalSize += f.Size
		}
		nonEmpty := fileops.HasNonEmptyDirs(files)
		a.ConfirmType = ConfirmDeleteBatch
		msg := fmt.Sprintf("Delete %d items (%s)? [y/N]", len(files), fsutil.FormatSize(totalSize))
		if nonEmpty > 0 {
			msg += fmt.Sprintf("\nWARNING: %d non-empty directories will be recursively deleted!", nonEmpty)
		}
		a.ConfirmMsg = msg
	}

	a.Mode = ModeConfirm
	return nil
}

func (a *App) openPreview() tea.Cmd {
	panel := a.Panels[a.ActivePanel]
	item := panel.GetCurrentItem()
	if item == nil {
		return nil
	}

	err := a.Preview.Load(item.Path, a.Width-4, a.Height-4, a.Theme.Name)
	if err != nil {
		a.ErrMsg = err.Error()
		return nil
	}

	a.Preview.IsOpen = true
	a.Mode = ModePreview
	return nil
}

func (a *App) handlePreviewMode(key string) tea.Cmd {
	switch key {
	case "esc", "q", "f3":
		a.Preview.IsOpen = false
		a.Mode = ModeNormal
	case "up", "k":
		a.Preview.ScrollUp()
	case "down", "j":
		a.Preview.ScrollDown()
	case "pageup", "ctrl+u":
		a.Preview.PageUp()
	case "pagedown", "ctrl+d":
		a.Preview.PageDown()
	}
	return nil
}

func (a *App) handleBookmarksMode(key string) tea.Cmd {
	bms := a.Bookmarks.List()

	switch key {
	case "esc", "q":
		a.Mode = ModeNormal
	case "1", "2", "3", "4", "5", "6", "7", "8", "9":
		idx := int(key[0] - '1')
		if idx >= 0 && idx < len(bms) {
			bm := bms[idx]
			panel := a.Panels[a.ActivePanel]
			panel.Path = fsutil.ExpandHome(bm.Path)
			a.Mode = ModeNormal
			return panels.LoadDirectoryCmd(panel.Path, panel.ShowHidden, a.ActivePanel)
		}
	}
	return nil
}

func (a *App) handleDiskUsageMode(key string) tea.Cmd {
	switch key {
	case "esc", "q":
		a.Mode = ModeNormal
	case "up", "k":
		if a.DiskCursor > 0 {
			a.DiskCursor--
		}
	case "down", "j":
		if a.DiskCursor < len(a.DiskItems)-1 {
			a.DiskCursor++
		}
	case "enter":
		if a.DiskCursor >= 0 && a.DiskCursor < len(a.DiskItems) {
			item := a.DiskItems[a.DiskCursor]
			if item.IsDir {
				panel := a.Panels[a.ActivePanel]
				panel.Path = item.Path
				a.Mode = ModeNormal
				return panels.LoadDirectoryCmd(panel.Path, panel.ShowHidden, a.ActivePanel)
			}
		}
	}
	return nil
}

func (a *App) handleConflictMode(key string) tea.Cmd {
	var action int
	switch key {
	case "o", "O":
		action = fileops.ConflictOverwrite
	case "s", "S":
		action = fileops.ConflictSkip
	case "a", "A":
		action = fileops.ConflictOverwriteAll
	case "n", "N":
		action = fileops.ConflictSkipAll
	case "r", "R":
		action = fileops.ConflictRename
	case "esc":
		action = fileops.ConflictCancel
	default:
		return nil
	}

	if a.ConflictRespChan != nil {
		select {
		case a.ConflictRespChan <- action:
		default:
		}
	}

	if action == fileops.ConflictCancel {
		a.Mode = ModeProgress
		return nil
	}

	a.Mode = ModeProgress
	return a.listenForConflicts()
}

func (a *App) handleConfirmMode(key string) tea.Cmd {
	switch key {
	case "y", "Y":
		return a.executeConfirm()
	case "n", "N", "esc":
		a.Mode = ModeNormal
		a.ConfirmMsg = ""
	}
	return nil
}

func (a *App) executeConfirm() tea.Cmd {
	panel := a.Panels[a.ActivePanel]
	files := panel.GetSelectedFiles()

	a.Mode = ModeProgress
	a.ProgressCancel = make(chan struct{}, 1)
	a.ProgressMsg = fileops.ProgressMsg{
		Current: 0,
		Total:   len(files),
	}

	progressChan := make(chan fileops.ProgressMsg, 100)

	go func() {
		err := fileops.DeleteFiles(files, progressChan, a.ProgressCancel)
		if err != nil {
			progressChan <- fileops.ProgressMsg{Error: err}
		}
		close(progressChan)
	}()

	return func() tea.Msg {
		for msg := range progressChan {
			return msg
		}
		return nil
	}
}

func (a *App) startDiskUsage() tea.Cmd {
	panel := a.Panels[a.ActivePanel]
	a.Mode = ModeDiskUsage
	a.DiskCursor = 0

	return func() tea.Msg {
		items, err := extra.AnalyzeDiskUsage(panel.Path)
		if err != nil {
			a.ErrMsg = err.Error()
			a.Mode = ModeNormal
			return nil
		}
		a.DiskItems = items
		return nil
	}
}

func (a *App) layout() {
	panelWidth := a.Width / 2
	if panelWidth < 40 {
		panelWidth = 40
	}

	for i := 0; i < 2; i++ {
		a.Panels[i].Width = panelWidth
		a.Panels[i].Height = a.Height - 2
	}

	a.Preview.Width = a.Width - 4
	a.Preview.Height = a.Height - 4
}

func (a *App) View() string {
	if a.Width < 80 || a.Height < 20 {
		return "Window too small. Please resize to at least 80x20."
	}

	switch a.Mode {
	case ModePreview:
		return a.viewPreview()
	case ModeProgress:
		return a.viewProgress()
	case ModeConflict:
		return a.viewWithOverlay(a.viewConflict())
	case ModeConfirm:
		return a.viewWithOverlay(a.viewConfirm())
	case ModeDiskUsage:
		return a.viewWithOverlay(a.viewDiskUsage())
	case ModeBookmarks:
		return a.viewWithOverlay(a.viewBookmarks())
	case ModeShell:
		if a.ShellOutput != "" {
			return a.viewWithOverlay(a.viewShellOutput())
		}
	}

	mainView := a.viewMain()

	if a.Mode == ModeInputPath || a.Mode == ModeSearch || a.Mode == ModeShell || a.Mode == ModeChmod || a.Mode == ModeRename || a.Mode == ModeNewFile || a.Mode == ModeNewDir {
		overlay := a.viewInput()
		return a.viewWithOverlay(mainView, overlay)
	}

	if a.ErrMsg != "" {
		overlay := a.viewError()
		return a.viewWithOverlay(mainView, overlay)
	}

	return mainView
}

func (a *App) viewConflict() string {
	lines := []string{
		"",
		" ⚠️  FILE CONFLICT ⚠️ ",
		"",
		fmt.Sprintf(" File: %s", a.ConflictFile),
		"",
		" Source:        Destination:",
	}

	if a.SrcInfo != nil {
		lines = append(lines, fmt.Sprintf(" Size: %-8s   Size: %s",
			fsutil.FormatSize(a.SrcInfo.Size),
			fsutil.FormatSize(a.DstInfo.Size)))
		lines = append(lines, fmt.Sprintf(" Modified: %s   Modified: %s",
			a.SrcInfo.ModTime.Format("2006-01-02 15:04"),
			a.DstInfo.ModTime.Format("2006-01-02 15:04")))
	}

	lines = append(lines, "")
	lines = append(lines, " [O] Overwrite  [S] Skip  [A] Overwrite All")
	lines = append(lines, " [N] Skip All   [R] Rename (auto suffix)")
	lines = append(lines, " [Esc] Cancel operation")
	lines = append(lines, "")

	return a.renderOverlay(strings.Join(lines, "\n"))
}

func (a *App) viewMain() string {
	left := a.Panels[0].View()
	right := a.Panels[1].View()

	panels := lipgloss.JoinHorizontal(lipgloss.Top, left, right)

	footer := a.renderFooter()

	return lipgloss.JoinVertical(lipgloss.Left, panels, footer)
}

func (a *App) renderFooter() string {
	keys := []string{
		"↑↓ Navigate",
		"Tab Switch",
		"Space Mark",
		"F2 Rename",
		"Shift+N New File",
		"Shift+D New Dir",
		"F5 Copy",
		"F6 Move",
		"F8 Delete",
		"F3 Preview",
		"Ctrl+A All",
		"Ctrl+H Hidden",
		"F7 Sort",
		"/ Path",
		"! Shell",
		"Q Quit",
	}

	footerText := strings.Join(keys, " | ")

	style := lipgloss.NewStyle().
		Background(lipgloss.Color("#1d2021")).
		Foreground(lipgloss.Color("#aaaaaa")).
		Width(a.Width).
		Align(lipgloss.Center)

	return style.Render(footerText)
}

func (a *App) viewPreview() string {
	main := a.viewMain()
	preview := a.Preview.View()
	return a.viewWithOverlay(main, preview)
}

func (a *App) viewProgress() string {
	main := a.viewMain()

	msg := a.ProgressMsg
	lines := []string{
		"",
		fmt.Sprintf(" Progress: %d/%d ", msg.Current, msg.Total),
		"",
	}

	if msg.Total > 0 && msg.FileName != "" {
		lines = append(lines, fmt.Sprintf(" File: %s", msg.FileName))
	}

	if msg.Percent > 0 {
		bar := renderProgressBar(msg.Percent, a.Width-20)
		lines = append(lines, fmt.Sprintf(" %s %.1f%%", bar, msg.Percent))
		lines = append(lines, fmt.Sprintf(" Speed: %s | ETA: %v",
			fileops.FormatSpeed(msg.Speed),
			msg.ETA.Round(time.Second)))
	}

	lines = append(lines, "", " Press ESC to cancel ")
	lines = append(lines, "")

	overlay := a.renderOverlay(strings.Join(lines, "\n"))
	return a.viewWithOverlay(main, overlay)
}

func (a *App) viewConfirm() string {
	lines := []string{
		"",
		" ⚠️  CONFIRM ACTION ⚠️ ",
		"",
	}
	for _, line := range strings.Split(a.ConfirmMsg, "\n") {
		lines = append(lines, " "+line)
	}
	lines = append(lines, "")
	lines = append(lines, " [Y] Yes  [N] No ")
	lines = append(lines, "")

	return a.renderOverlay(strings.Join(lines, "\n"))
}

func (a *App) viewInput() string {
	lines := []string{
		"",
		" " + a.InputLabel,
		"",
		" " + a.Input + "█",
		"",
	}

	if a.Mode == ModeSearch {
		recursive := "OFF"
		if a.SearchRecursive {
			recursive = "ON"
		}
		lines = append(lines, " Recursive: "+recursive+" (Ctrl+R to toggle)")
		lines = append(lines, "")
	}

	lines = append(lines, " [Enter] Confirm  [Esc] Cancel ")
	lines = append(lines, "")

	return a.renderOverlay(strings.Join(lines, "\n"))
}

func (a *App) viewBookmarks() string {
	lines := []string{
		"",
		" Bookmarks ",
		"",
	}

	bms := a.Bookmarks.List()
	for i, b := range bms {
		marker := " "
		if i == 0 {
			marker = ">"
		}
		lines = append(lines, fmt.Sprintf(" %s %d. %s -> %s", marker, i+1, b.Name, b.Path))
	}

	lines = append(lines, "")
	lines = append(lines, " [Number] Go  [Esc] Close ")
	lines = append(lines, "")

	return a.renderOverlay(strings.Join(lines, "\n"))
}

func (a *App) viewDiskUsage() string {
	lines := []string{
		"",
		" Disk Usage Analysis ",
		"",
	}

	for i, item := range a.DiskItems {
		marker := " "
		if i == a.DiskCursor {
			marker = ">"
		}

		name := item.Name
		if len(name) > 30 {
			name = name[:27] + "..."
		}

		bar := renderProgressBar(item.Percent, 20)
		lines = append(lines, fmt.Sprintf(" %s %-30s %8s %5.1f%% %s",
			marker, name, formatSize(item.Size), item.Percent, bar))
	}

	lines = append(lines, "")
	lines = append(lines, " [Enter] Go to dir  [Esc] Close ")
	lines = append(lines, "")

	return a.renderOverlay(strings.Join(lines, "\n"))
}

func (a *App) viewShellOutput() string {
	lines := []string{
		"",
		" Command Output ",
		"",
	}

	for _, line := range strings.Split(a.ShellOutput, "\n") {
		if len(line) > a.Width-10 {
			line = line[:a.Width-13] + "..."
		}
		lines = append(lines, " "+line)
		if len(lines) > a.Height-8 {
			lines = append(lines, " ... (truncated)")
			break
		}
	}

	lines = append(lines, "")
	lines = append(lines, " [Any key] Close ")
	lines = append(lines, "")

	return a.renderOverlay(strings.Join(lines, "\n"))
}

func (a *App) viewError() string {
	lines := []string{
		"",
		" ⚠️  Error ",
		"",
		" " + a.ErrMsg,
		"",
		" [Any key] Close ",
		"",
	}
	return a.renderOverlay(strings.Join(lines, "\n"))
}

func (a *App) renderOverlay(content string) string {
	maxWidth := a.Width - 10
	if maxWidth < 40 {
		maxWidth = 40
	}

	lines := strings.Split(content, "\n")
	for i, line := range lines {
		if len(line) > maxWidth-4 {
			lines[i] = line[:maxWidth-7] + "..."
		}
	}

	padded := strings.Join(lines, "\n")

	style := lipgloss.NewStyle().
		Border(lipgloss.DoubleBorder()).
		BorderForeground(lipgloss.Color("#555555")).
		Background(lipgloss.Color("#222222")).
		Foreground(lipgloss.Color("#ffffff")).
		Padding(0, 2)

	return style.Render(padded)
}

func (a *App) viewWithOverlay(base string, overlays ...string) string {
	result := base
	for _, overlay := range overlays {
		result = centerOverlay(result, overlay)
	}
	return result
}

func centerOverlay(base, overlay string) string {
	baseLines := strings.Split(base, "\n")
	overlayLines := strings.Split(overlay, "\n")

	if len(baseLines) == 0 {
		return base
	}

	baseWidth := len(stripANSI(baseLines[0]))
	overlayWidth := len(stripANSI(overlayLines[0]))
	if overlayWidth < len(overlayLines[0]) {
		overlayWidth = len(overlayLines[0])
	}

	startX := (baseWidth - overlayWidth) / 2
	if startX < 0 {
		startX = 0
	}

	startY := (len(baseLines) - len(overlayLines)) / 2
	if startY < 0 {
		startY = 0
	}

	for i, line := range overlayLines {
		y := startY + i
		if y >= len(baseLines) {
			break
		}

		prefix := strings.Repeat(" ", startX)
		if len(baseLines[y]) >= startX {
			baseLines[y] = baseLines[y][:startX] + line
		} else {
			baseLines[y] = prefix + line
		}
	}

	return strings.Join(baseLines, "\n")
}

func stripANSI(s string) string {
	result := ""
	i := 0
	for i < len(s) {
		if s[i] == 0x1b && i+1 < len(s) && s[i+1] == '[' {
			j := i + 2
			for j < len(s) {
				if (s[j] >= 0x40 && s[j] <= 0x7a) && s[j] != '[' {
					break
				}
				j++
			}
			if j < len(s) {
				i = j + 1
				continue
			}
		}
		result += string(s[i])
		i++
	}
	return result
}

func renderProgressBar(percent float64, width int) string {
	if width <= 0 {
		return ""
	}

	filled := int(percent / 100 * float64(width))
	if filled < 0 {
		filled = 0
	}
	if filled > width {
		filled = width
	}

	bar := "[" + strings.Repeat("█", filled) + strings.Repeat("░", width-filled) + "]"
	return bar
}

func formatSize(size int64) string {
	return extra.FormatSize(size)
}
