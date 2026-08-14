package fs

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

type FileType int

const (
	TypeRegular FileType = iota
	TypeDir
	TypeSymlink
	TypeExecutable
)

type FileItem struct {
	Name     string
	Path     string
	Size     int64
	ModTime  time.Time
	Mode     fs.FileMode
	Type     FileType
	IsDir    bool
	IsMarked bool
}

type Directory struct {
	Path      string
	Files     []FileItem
	FileCount int
	DirCount  int
	TotalSize int64
}

type SortType int

const (
	SortByName SortType = iota
	SortBySize
	SortByTime
	SortByType
)

func FormatSize(size int64) string {
	const unit = 1000
	if size < unit {
		return fmt.Sprintf("%d B", size)
	}
	div, exp := int64(unit), 0
	for n := size / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(size)/float64(div), "KMGTPE"[exp])
}

func FormatMode(mode fs.FileMode) string {
	perms := [9]byte{'-', '-', '-', '-', '-', '-', '-', '-', '-'}

	if mode&0400 != 0 {
		perms[0] = 'r'
	}
	if mode&0200 != 0 {
		perms[1] = 'w'
	}
	if mode&0100 != 0 {
		perms[2] = 'x'
	}
	if mode&0040 != 0 {
		perms[3] = 'r'
	}
	if mode&0020 != 0 {
		perms[4] = 'w'
	}
	if mode&0010 != 0 {
		perms[5] = 'x'
	}
	if mode&0004 != 0 {
		perms[6] = 'r'
	}
	if mode&0002 != 0 {
		perms[7] = 'w'
	}
	if mode&0001 != 0 {
		perms[8] = 'x'
	}

	result := string(perms[:])
	if mode.IsDir() {
		result = "d" + result
	} else if mode&os.ModeSymlink != 0 {
		result = "l" + result
	} else {
		result = "-" + result
	}

	return result
}

func getFileType(info os.DirEntry, path string) FileType {
	if info.IsDir() {
		return TypeDir
	}
	if info.Type()&os.ModeSymlink != 0 {
		return TypeSymlink
	}
	realInfo, err := info.Info()
	if err == nil && realInfo.Mode()&0111 != 0 {
		return TypeExecutable
	}
	return TypeRegular
}

func ReadDirectory(dirPath string, showHidden bool) (*Directory, error) {
	absPath, err := filepath.Abs(dirPath)
	if err != nil {
		return nil, err
	}

	entries, err := os.ReadDir(absPath)
	if err != nil {
		return nil, err
	}

	dir := &Directory{
		Path: absPath,
	}

	var wg sync.WaitGroup
	filesChan := make(chan FileItem, len(entries))
	sem := make(chan struct{}, 100)

	for _, entry := range entries {
		name := entry.Name()
		if !showHidden && strings.HasPrefix(name, ".") {
			continue
		}

		wg.Add(1)
		sem <- struct{}{}

		go func(e os.DirEntry, absP, n string) {
			defer wg.Done()
			defer func() { <-sem }()

			fullPath := filepath.Join(absP, n)
			var size int64
			var modTime time.Time
			var mode fs.FileMode

			info, err := e.Info()
			if err == nil {
				size = info.Size()
				modTime = info.ModTime()
				mode = info.Mode()
			}

			fType := getFileType(e, fullPath)

			item := FileItem{
				Name:     n,
				Path:     fullPath,
				Size:     size,
				ModTime:  modTime,
				Mode:     mode,
				Type:     fType,
				IsDir:    e.IsDir(),
				IsMarked: false,
			}

			filesChan <- item
		}(entry, absPath, name)
	}

	go func() {
		wg.Wait()
		close(filesChan)
	}()

	dir.Files = make([]FileItem, 0, len(entries))
	for item := range filesChan {
		dir.Files = append(dir.Files, item)
		if item.IsDir {
			dir.DirCount++
		} else {
			dir.FileCount++
			dir.TotalSize += item.Size
		}
	}

	return dir, nil
}

func SortFiles(files []FileItem, sortType SortType, ascending bool, dirsFirst bool) {
	less := func(i, j int) bool {
		a, b := files[i], files[j]

		if dirsFirst {
			if a.IsDir != b.IsDir {
				return a.IsDir
			}
		}

		var cmp int
		switch sortType {
		case SortByName:
			cmp = strings.Compare(strings.ToLower(a.Name), strings.ToLower(b.Name))
		case SortBySize:
			if a.Size < b.Size {
				cmp = -1
			} else if a.Size > b.Size {
				cmp = 1
			}
		case SortByTime:
			if a.ModTime.Before(b.ModTime) {
				cmp = -1
			} else if a.ModTime.After(b.ModTime) {
				cmp = 1
			}
		case SortByType:
			extA := strings.ToLower(filepath.Ext(a.Name))
			extB := strings.ToLower(filepath.Ext(b.Name))
			cmp = strings.Compare(extA, extB)
			if cmp == 0 {
				cmp = strings.Compare(strings.ToLower(a.Name), strings.ToLower(b.Name))
			}
		}

		if !ascending {
			cmp = -cmp
		}
		return cmp < 0
	}

	sort.Slice(files, less)
}

func ParentDir(path string) string {
	parent := filepath.Dir(path)
	if parent == path {
		return path
	}
	return parent
}

func ExpandHome(path string) string {
	if strings.HasPrefix(path, "~") {
		home, err := os.UserHomeDir()
		if err == nil {
			return strings.Replace(path, "~", home, 1)
		}
	}
	return path
}

func JoinPath(base, rel string) string {
	if filepath.IsAbs(rel) {
		return rel
	}
	return filepath.Join(base, rel)
}

func IsDir(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return info.IsDir()
}

func Exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func UniqueName(destPath string) string {
	if !Exists(destPath) {
		return destPath
	}

	dir := filepath.Dir(destPath)
	name := filepath.Base(destPath)
	ext := filepath.Ext(name)
	baseName := strings.TrimSuffix(name, ext)

	for i := 1; ; i++ {
		newName := fmt.Sprintf("%s_%d%s", baseName, i, ext)
		newPath := filepath.Join(dir, newName)
		if !Exists(newPath) {
			return newPath
		}
	}
}
