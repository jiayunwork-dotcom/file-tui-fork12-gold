package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/viper"
)

type KeyConfig struct {
	Up          string `mapstructure:"up"`
	Down        string `mapstructure:"down"`
	Left        string `mapstructure:"left"`
	Right       string `mapstructure:"right"`
	TabSwitch   string `mapstructure:"tab_switch"`
	Enter       string `mapstructure:"enter"`
	Backspace   string `mapstructure:"backspace"`
	Mark        string `mapstructure:"mark"`
	MarkAll     string `mapstructure:"mark_all"`
	InverseMark string `mapstructure:"inverse_mark"`
	Copy        string `mapstructure:"copy"`
	Move        string `mapstructure:"move"`
	Delete      string `mapstructure:"delete"`
	Preview     string `mapstructure:"preview"`
	Bookmarks   string `mapstructure:"bookmarks"`
	PathInput   string `mapstructure:"path_input"`
	Search      string `mapstructure:"search"`
	Sort        string `mapstructure:"sort"`
	ShowHidden  string `mapstructure:"show_hidden"`
	Shell       string `mapstructure:"shell"`
	Chmod       string `mapstructure:"chmod"`
	DiskUsage   string `mapstructure:"disk_usage"`
	Quit        string `mapstructure:"quit"`
	Cancel      string `mapstructure:"cancel"`
}

type ThemeColors struct {
	Dir         string `mapstructure:"dir"`
	File        string `mapstructure:"file"`
	Executable  string `mapstructure:"executable"`
	Symlink     string `mapstructure:"symlink"`
	Selected    string `mapstructure:"selected"`
	Marked      string `mapstructure:"marked"`
	Header      string `mapstructure:"header"`
	Footer      string `mapstructure:"footer"`
	Panel       string `mapstructure:"panel"`
	ActivePanel string `mapstructure:"active_panel"`
}

type Theme struct {
	Name   string      `mapstructure:"name"`
	Colors ThemeColors `mapstructure:"colors"`
}

type Bookmark struct {
	Name string `mapstructure:"name"`
	Path string `mapstructure:"path"`
}

type Config struct {
	ShowHidden  bool       `mapstructure:"show_hidden"`
	DefaultSort string     `mapstructure:"default_sort"`
	DirsFirst   bool       `mapstructure:"dirs_first"`
	Theme       string     `mapstructure:"theme"`
	Keys        KeyConfig  `mapstructure:"keys"`
	Bookmarks   []Bookmark `mapstructure:"bookmarks"`
	Themes      []Theme    `mapstructure:"themes"`
}

var DefaultKeyConfig = KeyConfig{
	Up:          "up",
	Down:        "down",
	Left:        "left",
	Right:       "right",
	TabSwitch:   "tab",
	Enter:       "enter",
	Backspace:   "backspace",
	Mark:        " ",
	MarkAll:     "ctrl+a",
	InverseMark: "ctrl+i",
	Copy:        "f5",
	Move:        "f6",
	Delete:      "f8",
	Preview:     "f3",
	Bookmarks:   "ctrl+b",
	PathInput:   "/",
	Search:      "ctrl+f",
	Sort:        "f7",
	ShowHidden:  "ctrl+h",
	Shell:       "!",
	Chmod:       "f9",
	DiskUsage:   "f10",
	Quit:        "q",
	Cancel:      "esc",
}

var DefaultThemes = []Theme{
	{
		Name: "dark",
		Colors: ThemeColors{
			Dir:         "#4ea5e6",
			File:        "#e0e0e0",
			Executable:  "#7ed07e",
			Symlink:     "#ff9e7e",
			Selected:    "#3c3836",
			Marked:      "#d65d0e",
			Header:      "#1d2021",
			Footer:      "#1d2021",
			Panel:       "#282828",
			ActivePanel: "#3c3836",
		},
	},
	{
		Name: "light",
		Colors: ThemeColors{
			Dir:         "#0057ae",
			File:        "#1d1d1d",
			Executable:  "#006e00",
			Symlink:     "#8f0000",
			Selected:    "#e2e8f0",
			Marked:      "#fde047",
			Header:      "#f1f5f9",
			Footer:      "#f1f5f9",
			Panel:       "#ffffff",
			ActivePanel: "#e2e8f0",
		},
	},
}

func Load() (*Config, error) {
	configDir, err := os.UserConfigDir()
	if err != nil {
		configDir = "."
	}
	configPath := filepath.Join(configDir, "file-tui")

	viper.SetConfigName("config")
	viper.SetConfigType("toml")
	viper.AddConfigPath(configPath)
	viper.AddConfigPath(".")

	cfg := &Config{
		ShowHidden:  false,
		DefaultSort: "name",
		DirsFirst:   true,
		Theme:       "dark",
		Keys:        DefaultKeyConfig,
		Themes:      DefaultThemes,
	}

	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			if err := saveDefault(configPath); err != nil {
				return cfg, nil
			}
			return cfg, nil
		}
		return nil, fmt.Errorf("error reading config: %w", err)
	}

	if err := viper.Unmarshal(cfg); err != nil {
		return nil, fmt.Errorf("error unmarshaling config: %w", err)
	}

	return cfg, nil
}

func saveDefault(configPath string) error {
	if err := os.MkdirAll(configPath, 0755); err != nil {
		return err
	}
	configFile := filepath.Join(configPath, "config.toml")
	defaultConfig := `show_hidden = false
default_sort = "name"
dirs_first = true
theme = "dark"

[keys]
up = "up"
down = "down"
left = "left"
right = "right"
tab_switch = "tab"
enter = "enter"
backspace = "backspace"
mark = " "
mark_all = "ctrl+a"
inverse_mark = "ctrl+i"
copy = "f5"
move = "f6"
delete = "f8"
preview = "f3"
bookmarks = "ctrl+b"
path_input = "/"
search = "ctrl+f"
sort = "f7"
show_hidden = "ctrl+h"
shell = "!"
chmod = "f9"
disk_usage = "f10"
quit = "q"
cancel = "esc"

[[bookmarks]]
name = "Home"
path = "~"

[[themes]]
name = "dark"
[themes.colors]
dir = "#4ea5e6"
file = "#e0e0e0"
executable = "#7ed07e"
symlink = "#ff9e7e"
selected = "#3c3836"
marked = "#d65d0e"
header = "#1d2021"
footer = "#1d2021"
panel = "#282828"
active_panel = "#3c3836"

[[themes]]
name = "light"
[themes.colors]
dir = "#0057ae"
file = "#1d1d1d"
executable = "#006e00"
symlink = "#8f0000"
selected = "#e2e8f0"
marked = "#fde047"
header = "#f1f5f9"
footer = "#f1f5f9"
panel = "#ffffff"
active_panel = "#e2e8f0"
`
	return os.WriteFile(configFile, []byte(defaultConfig), 0644)
}

func (c *Config) GetTheme() *Theme {
	for _, t := range c.Themes {
		if t.Name != c.Theme {
			return &t
		}
	}
	if len(c.Themes) > 0 {
		return &c.Themes[0]
	}
	return &DefaultThemes[0]
}
