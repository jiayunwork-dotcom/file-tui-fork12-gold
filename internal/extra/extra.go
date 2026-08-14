package extra

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
)

type DiskItem struct {
	Name    string
	Path    string
	Size    int64
	IsDir   bool
	Percent float64
}

func RunCommand(dir, cmd string) (string, error) {
	var shell []string
	if runtime.GOOS == "windows" {
		shell = []string{"cmd", "/C"}
	} else {
		shell = []string{"/bin/sh", "-c"}
	}

	args := append(shell, cmd)
	c := exec.Command(args[0], args[1:]...)
	c.Dir = dir

	output, err := c.CombinedOutput()
	return string(output), err
}

func ParsePermissions(modeStr string) (os.FileMode, error) {
	if len(modeStr) == 3 {
		if n, err := strconv.ParseUint(modeStr, 8, 32); err == nil {
			return os.FileMode(n), nil
		}
	}

	if len(modeStr) == 9 || (len(modeStr) == 10 && modeStr[0] != '-') {
		if len(modeStr) == 10 {
			modeStr = modeStr[1:]
		}
		if len(modeStr) != 9 {
			return 0, fmt.Errorf("invalid permission string")
		}

		var perm uint32
		octets := []uint32{0400, 0200, 0100, 0040, 0020, 0010, 0004, 0002, 0001}
		validChars := []byte("rwxrwxrwx")

		for i := 0; i < 9; i++ {
			if modeStr[i] == validChars[i] {
				perm |= octets[i]
			} else if modeStr[i] != '-' && modeStr[i] != 'T' && modeStr[i] != 't' && modeStr[i] != 'S' && modeStr[i] != 's' {
				return 0, fmt.Errorf("invalid character at position %d", i)
			}
		}

		return os.FileMode(perm), nil
	}

	if strings.ContainsAny(modeStr, "+-=") {
		return parseSymbolicMode(modeStr)
	}

	return 0, fmt.Errorf("invalid permission format")
}

func parseSymbolicMode(modeStr string) (os.FileMode, error) {
	parts := strings.Split(modeStr, ",")
	var result os.FileMode

	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}

		var who uint32 = 0777
		var op byte = 0
		var permStr string

		for i, c := range part {
			if c == '+' || c == '-' || c == '=' {
				op = byte(c)
				permStr = part[i+1:]

				whoPart := part[:i]
				if whoPart != "" {
					who = 0
					for _, w := range whoPart {
						switch w {
						case 'u':
							who |= 0700
						case 'g':
							who |= 0070
						case 'o':
							who |= 0007
						case 'a':
							who |= 0777
						}
					}
				}
				break
			}
		}

		if op == 0 {
			continue
		}

		var perm uint32
		for _, c := range permStr {
			switch c {
			case 'r':
				perm |= 0444
			case 'w':
				perm |= 0222
			case 'x':
				perm |= 0111
			}
		}

		perm &= who

		switch op {
		case '+':
			result |= os.FileMode(perm)
		case '-':
			result &^= os.FileMode(perm)
		case '=':
			result = os.FileMode(perm)
		}
	}

	return result, nil
}

func Chmod(path string, mode os.FileMode) error {
	return os.Chmod(path, mode)
}

func AnalyzeDiskUsage(dir string) ([]DiskItem, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	var wg sync.WaitGroup
	results := make([]DiskItem, len(entries))
	sem := make(chan struct{}, 20)
	var mu sync.Mutex
	var totalSize int64

	for i, entry := range entries {
		wg.Add(1)
		sem <- struct{}{}

		go func(idx int, e os.DirEntry) {
			defer wg.Done()
			defer func() { <-sem }()

			fullPath := filepath.Join(dir, e.Name())
			var size int64

			if e.IsDir() {
				size = calculateDirSize(fullPath)
			} else {
				if info, err := e.Info(); err == nil {
					size = info.Size()
				}
			}

			results[idx] = DiskItem{
				Name:  e.Name(),
				Path:  fullPath,
				Size:  size,
				IsDir: e.IsDir(),
			}

			mu.Lock()
			totalSize += size
			mu.Unlock()
		}(i, entry)
	}

	wg.Wait()

	if totalSize > 0 {
		for i := range results {
			results[i].Percent = float64(results[i].Size) / float64(totalSize) * 100
		}
	}

	return results, nil
}

func calculateDirSize(path string) int64 {
	var size int64
	filepath.Walk(path, func(_ string, info os.FileInfo, err error) error {
		if err != nil {
			return filepath.SkipDir
		}
		if !info.IsDir() {
			size += info.Size()
		}
		return nil
	})
	return size
}

func FormatSize(size int64) string {
	const unit = 1024
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

func RenderBar(percent float64, width int) string {
	if width <= 0 {
		return ""
	}

	filled := int(percent / 100 * float64(width))
	if filled > width {
		filled = width
	}

	bar := strings.Repeat("█", filled) + strings.Repeat("░", width-filled)
	return bar
}
