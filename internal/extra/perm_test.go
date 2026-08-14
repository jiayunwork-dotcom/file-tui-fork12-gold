package extra

import "testing"

func TestParsePermissions_FourDigitOctal(t *testing.T) {
	mode, err := ParsePermissions("0755")
	if err != nil {
		t.Fatalf("ParsePermissions(0755) err=%v", err)
	}
	if mode&0o755 != 0o755 {
		t.Fatalf("ParsePermissions(0755)=%o, want 0755 bits", mode)
	}
}
