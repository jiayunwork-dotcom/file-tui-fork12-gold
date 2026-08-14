package navigation

import (
	"os"
	"path/filepath"
	"strings"

	"file-tui/internal/config"
)

type SearchResult struct {
	Name  string
	Path  string
	IsDir bool
}

func SearchFiles(rootDir, query string, recursive bool) []SearchResult {
	query = strings.ToLower(query)
	var results []SearchResult

	if !recursive {
		entries, err := os.ReadDir(rootDir)
		if err != nil {
			return results
		}
		for _, e := range entries {
			if strings.Contains(strings.ToLower(e.Name()), query) {
				results = append(results, SearchResult{
					Name:  e.Name(),
					Path:  filepath.Join(rootDir, e.Name()),
					IsDir: e.IsDir(),
				})
			}
		}
		return results
	}

	filepath.WalkDir(rootDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return filepath.SkipDir
		}
		if path == rootDir {
			return nil
		}
		if strings.Contains(strings.ToLower(d.Name()), query) {
			results = append(results, SearchResult{
				Name:  d.Name(),
				Path:  path,
				IsDir: d.IsDir(),
			})
		}
		if !d.IsDir() {
			return nil
		}
		return nil
	})

	return results
}

func FilterFiles(files []string, query string) []string {
	query = strings.ToLower(query)
	var filtered []string
	for _, f := range files {
		if strings.Contains(f, query) {
			filtered = append(filtered, f)
		}
	}
	return filtered
}

func CompletePath(input string) []string {
	dir := filepath.Dir(input)
	prefix := filepath.Base(input)

	if dir == "" {
		dir = "."
	}

	if input == "" {
		dir = "."
		prefix = ""
	}

	if strings.HasSuffix(input, "/") || strings.HasSuffix(input, "\\") {
		dir = input
		prefix = ""
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return []string{}
	}

	var completions []string
	for _, e := range entries {
		name := e.Name()
		if prefix == "" || strings.HasPrefix(strings.ToLower(name), strings.ToLower(prefix)) {
			fullPath := filepath.Join(dir, name)
			if e.IsDir() {
				completions = append(completions, fullPath+"/")
			} else {
				completions = append(completions, fullPath)
			}
		}
	}

	return completions
}

type Bookmarks struct {
	items []config.Bookmark
}

func NewBookmarks(bms []config.Bookmark) *Bookmarks {
	items := make([]config.Bookmark, 0, len(bms))
	for _, b := range bms {
		items = append(items, config.Bookmark{
			Name: b.Name,
			Path: expandHome(b.Path),
		})
	}
	return &Bookmarks{items: items}
}

func (b *Bookmarks) List() []config.Bookmark {
	return b.items
}

func (b *Bookmarks) Get(index int) *config.Bookmark {
	if index >= 0 && index < len(b.items) {
		return &b.items[index]
	}
	return nil
}

func (b *Bookmarks) Add(name, path string) bool {
	if len(b.items) >= 20 {
		return false
	}
	b.items = append(b.items, config.Bookmark{
		Name: name,
		Path: expandHome(path),
	})
	return true
}

func (b *Bookmarks) Remove(index int) bool {
	if index < 0 || index >= len(b.items) {
		return false
	}
	b.items = append(b.items[:index], b.items[index+1:]...)
	return true
}

func (b *Bookmarks) ToConfig() []config.Bookmark {
	return b.items
}

func expandHome(path string) string {
	if strings.HasPrefix(path, "~") {
		home, err := os.UserHomeDir()
		if err == nil {
			return strings.Replace(path, "~", home, 1)
		}
	}
	return path
}
