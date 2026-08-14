package fileops

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"file-tui/internal/fs"
)

const (
	ConflictOverwrite = iota
	ConflictSkip
	ConflictOverwriteAll
	ConflictSkipAll
	ConflictRename
	ConflictCancel
)

type ConflictRequestMsg struct {
	Src string
	Dst string
}

type ConflictResponseMsg struct {
	Action int
}

const LargeFileThreshold = 10 * 1024 * 1024

type ProgressMsg struct {
	Percent  float64
	Speed    float64
	ETA      time.Duration
	Current  int
	Total    int
	FileName string
	Done     bool
	Error    error
}

type CancelOpMsg struct{}

type ConflictHandler func(src, dst string) int

type Options struct {
	OverwriteAll      bool
	SkipAll           bool
	Cancel            bool
	Canceled          bool
	ConflictRequest   chan ConflictRequestMsg
	ConflictResponse  <-chan int
}

func CopyDirectory(srcDir, dstDir string, files []fs.FileItem, opts *Options, progressChan chan<- ProgressMsg, cancelChan <-chan struct{}) error {
	if opts == nil {
		opts = &Options{}
	}

	for i, f := range files {
		if opts.Canceled {
			return nil
		}

		dst := filepath.Join(dstDir, f.Name)

		if fs.Exists(dst) {
			action := ConflictOverwrite
			if opts.OverwriteAll {
				action = ConflictOverwrite
			} else if opts.SkipAll {
				action = ConflictSkip
			} else if opts.ConflictRequest != nil && opts.ConflictResponse != nil {
				opts.ConflictRequest <- ConflictRequestMsg{Src: f.Path, Dst: dst}
				action = <-opts.ConflictResponse
			}

			switch action {
			case ConflictSkip:
				continue
			case ConflictSkipAll:
				opts.SkipAll = true
				continue
			case ConflictCancel:
				return nil
			case ConflictRename:
				dst = fs.UniqueName(dst)
			case ConflictOverwriteAll:
				opts.OverwriteAll = true
			}
		}

		progressChan <- ProgressMsg{
			Current:  i + 1,
			Total:    len(files),
			FileName: f.Name,
		}

		var err error
		if f.IsDir {
			err = copyDir(f.Path, dst, opts, progressChan, cancelChan)
		} else {
			err = copyFile(f.Path, dst, opts, progressChan, cancelChan, i+1, len(files), f.Name)
		}

		if err != nil {
			progressChan <- ProgressMsg{Error: err}
			return err
		}
	}

	progressChan <- ProgressMsg{Done: true}
	return nil
}

func MoveDirectory(srcDir, dstDir string, files []fs.FileItem, opts *Options, progressChan chan<- ProgressMsg, cancelChan <-chan struct{}) error {
	if opts == nil {
		opts = &Options{}
	}

	for i, f := range files {
		if opts.Canceled {
			return nil
		}

		dst := filepath.Join(dstDir, f.Name)

		if fs.Exists(dst) {
			action := ConflictOverwrite
			if opts.OverwriteAll {
				action = ConflictOverwrite
			} else if opts.SkipAll {
				action = ConflictSkip
			} else if opts.ConflictRequest != nil && opts.ConflictResponse != nil {
				opts.ConflictRequest <- ConflictRequestMsg{Src: f.Path, Dst: dst}
				action = <-opts.ConflictResponse
			}

			switch action {
			case ConflictSkip:
				continue
			case ConflictSkipAll:
				opts.SkipAll = true
				continue
			case ConflictCancel:
				return nil
			case ConflictRename:
				dst = fs.UniqueName(dst)
			case ConflictOverwriteAll:
				opts.OverwriteAll = true
			}
		}

		progressChan <- ProgressMsg{
			Current:  i + 1,
			Total:    len(files),
			FileName: f.Name,
		}

		err := moveItem(f.Path, dst, f.Size, opts, progressChan, cancelChan, i+1, len(files), f.Name)
		if err != nil {
			progressChan <- ProgressMsg{Error: err}
			return err
		}
	}

	progressChan <- ProgressMsg{Done: true}
	return nil
}

func DeleteFiles(files []fs.FileItem, progressChan chan<- ProgressMsg, cancelChan <-chan struct{}) error {
	for i, f := range files {
		select {
		case <-cancelChan:
			return nil
		default:
		}

		progressChan <- ProgressMsg{
			Current:  i + 1,
			Total:    len(files),
			FileName: f.Name,
		}

		var err error
		if f.IsDir {
			err = os.RemoveAll(f.Path)
		} else {
			err = os.Remove(f.Path)
		}

		if err != nil {
			return err
		}
	}

	progressChan <- ProgressMsg{Done: true}
	return nil
}

func copyFile(src, dst string, opts *Options, progressChan chan<- ProgressMsg, cancelChan <-chan struct{}, current, total int, name string) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	dstFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer dstFile.Close()

	info, err := srcFile.Stat()
	if err != nil {
		return err
	}

	if info.Size() < LargeFileThreshold {
		_, err = io.Copy(dstFile, srcFile)
		return err
	}

	return copyWithProgress(srcFile, dstFile, info.Size(), progressChan, cancelChan, current, total, name)
}

func copyWithProgress(src, dst *os.File, size int64, progressChan chan<- ProgressMsg, cancelChan <-chan struct{}, current, total int, name string) error {
	buf := make([]byte, 64*1024)
	var copied int64
	startTime := time.Now()

	for {
		select {
		case <-cancelChan:
			return nil
		default:
		}

		n, err := src.Read(buf)
		if err != nil && err != io.EOF {
			return err
		}

		if n == 0 {
			break
		}

		_, err = dst.Write(buf[:n])
		if err != nil {
			return err
		}

		copied += int64(n)
		elapsed := time.Since(startTime)
		if elapsed > 100*time.Millisecond || copied == size {
			percent := float64(copied) / float64(size) * 100
			speed := float64(copied) / elapsed.Seconds()
			eta := time.Duration(0)
			if speed > 0 {
				remaining := float64(size-copied) / speed
				eta = time.Duration(remaining * float64(time.Second))
			}

			progressChan <- ProgressMsg{
				Percent:  percent,
				Speed:    speed,
				ETA:      eta,
				Current:  current,
				Total:    total,
				FileName: name,
			}
		}
	}

	return nil
}

func copyDir(src, dst string, opts *Options, progressChan chan<- ProgressMsg, cancelChan <-chan struct{}) error {
	info, err := os.Stat(src)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(dst, info.Mode()); err != nil {
		return err
	}

	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		select {
		case <-cancelChan:
			return nil
		default:
		}

		srcPath := filepath.Join(src, entry.Name())
		dstPath := filepath.Join(dst, entry.Name())

		if fs.Exists(dstPath) {
			action := ConflictOverwrite
			if opts.OverwriteAll {
				action = ConflictOverwrite
			} else if opts.SkipAll {
				action = ConflictSkip
			}

			switch action {
			case ConflictSkip:
				continue
			}
		}

		if entry.IsDir() {
			if err := copyDir(srcPath, dstPath, opts, progressChan, cancelChan); err != nil {
				return err
			}
		} else {
			if err := copyFile(srcPath, dstPath, opts, progressChan, cancelChan, 0, 0, entry.Name()); err != nil {
				return err
			}
		}
	}

	return nil
}

func moveItem(src, dst string, size int64, opts *Options, progressChan chan<- ProgressMsg, cancelChan <-chan struct{}, current, total int, name string) error {
	err := os.Rename(src, dst)
	if err == nil {
		return nil
	}

	if err := copyFile(src, dst, opts, progressChan, cancelChan, current, total, name); err != nil {
		return err
	}

	return os.RemoveAll(src)
}

func CountDirContent(path string) (fileCount, dirCount int) {
	entries, _ := os.ReadDir(path)
	for _, e := range entries {
		if e.IsDir() {
			dirCount++
			fc, dc := CountDirContent(filepath.Join(path, e.Name()))
			fileCount += fc
			dirCount += dc
		} else {
			fileCount++
		}
	}
	return
}

func HasNonEmptyDirs(files []fs.FileItem) int {
	count := 0
	for _, f := range files {
		if f.IsDir {
			entries, err := os.ReadDir(f.Path)
			if err == nil && len(entries) > 0 {
				count++
			}
		}
	}
	return count
}

func FormatSpeed(bytesPerSec float64) string {
	const unit = 1024
	if bytesPerSec < unit {
		return fmt.Sprintf("%.1f B/s", bytesPerSec)
	}
	div, exp := float64(unit), 0
	for n := bytesPerSec / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB/s", bytesPerSec/div, "KMGTPE"[exp])
}

func ProgressCmd(op func(chan<- ProgressMsg, <-chan struct{}) error) tea.Cmd {
	return func() tea.Msg {
		progressChan := make(chan ProgressMsg, 100)
		cancelChan := make(chan struct{})

		go func() {
			err := op(progressChan, cancelChan)
			if err != nil {
				progressChan <- ProgressMsg{Error: err}
			}
			close(progressChan)
		}()

		for range progressChan {
			// This won't actually send messages; this is a factory
		}
		return nil
	}
}
