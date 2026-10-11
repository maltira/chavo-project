package color

import "testing"

func TestNormalize(t *testing.T) {
	if c, ok := Normalize("#34d399"); !ok || c != "#34D399" {
		t.Fatalf("valid: %q %v", c, ok)
	}
	for _, bad := range []string{"", "#FFF", "34D399", "#34D399FF", "red", "#GGGGGG", " #34D399"} {
		if _, ok := Normalize(bad); ok {
			t.Errorf("%q accepted", bad)
		}
	}
}
