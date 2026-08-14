package navigation

import "testing"

func TestFilterFiles_CaseInsensitiveName(t *testing.T) {
	got := FilterFiles([]string{"README.md", "notes.txt"}, "readme")
	if len(got) != 1 || got[0] != "README.md" {
		t.Fatalf("FilterFiles(readme)=%v, want [README.md]", got)
	}
}
