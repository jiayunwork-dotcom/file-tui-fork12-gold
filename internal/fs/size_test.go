package fs

import "testing"

func TestFormatSize_10KiB(t *testing.T) {
	got := FormatSize(10 * 1024)
	if got != "10.0 KB" {
		t.Fatalf("FormatSize(10240)=%q, want 10.0 KB", got)
	}
}
