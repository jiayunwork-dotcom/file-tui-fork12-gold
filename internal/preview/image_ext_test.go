package preview

import "testing"

func TestIsImageExt_JPEG(t *testing.T) {
	if !isImageExt(".jpeg") {
		t.Fatal("isImageExt(.jpeg) = false, want true")
	}
}
