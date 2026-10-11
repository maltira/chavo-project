package avatarshape

import (
	"errors"
	"strings"
	"testing"
)

func TestCatalog(t *testing.T) {
	got := Shapes()
	if len(got) != 27 {
		t.Fatalf("shapes: %d %v", len(got), got)
	}
	seen := map[string]bool{}
	for _, s := range got {
		if seen[s] {
			t.Fatalf("duplicate %q", s)
		}
		seen[s] = true
		if s != strings.ToLower(s) || strings.ContainsAny(s, " _") {
			t.Fatalf("%q is not lowercase kebab-case", s)
		}
		if !Known(s) {
			t.Fatalf("%q must be known", s)
		}
	}
	for _, want := range []string{"circle", "very-sunny", "12-sided-cookie", "4-leaf-clover", "soft-boom", "pixel-circle", "bun"} {
		if !seen[want] {
			t.Fatalf("missing %q", want)
		}
	}
	if Known("Circle") || Known("hexagon") || Known("") {
		t.Fatal("unknown shape accepted")
	}
	Shapes()[0] = "x"
	if Shapes()[0] != "circle" {
		t.Fatal("Shapes returned shared slice")
	}
}

func TestValidate(t *testing.T) {
	v, err := Validate("4-leaf-clover", "#34d399")
	if err != nil || v.Shape != "4-leaf-clover" || v.Color != "#34D399" {
		t.Fatalf("valid: %+v %v", v, err)
	}
	if _, err := Validate("hexagon", "#000000"); !errors.Is(err, ErrUnknownShape) {
		t.Fatalf("unknown shape: %v", err)
	}
	for _, c := range []string{"", "red", "#FFF", "#34D399FF"} {
		if _, err := Validate("circle", c); !errors.Is(err, ErrInvalidColor) {
			t.Errorf("color %q: %v", c, err)
		}
	}
}
