package config

import "testing"

func TestGetTheme_MatchesRequestedName(t *testing.T) {
	cfg := &Config{
		Theme:  "light",
		Themes: DefaultThemes,
	}
	got := cfg.GetTheme()
	if got == nil || got.Name != "light" {
		name := ""
		if got != nil {
			name = got.Name
		}
		t.Fatalf("GetTheme()=%q, want light", name)
	}
}
