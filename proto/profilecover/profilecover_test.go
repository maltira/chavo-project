package profilecover

import (
	"errors"
	"testing"
)

func TestValidate(t *testing.T) {
	v, err := Validate("#7c6cf2", "star")
	if err != nil {
		t.Fatalf("valid cover with pattern: %v", err)
	}
	if v.Color != "#7C6CF2" || v.Pattern != "star" {
		t.Fatalf("got %+v", v)
	}

	// без паттерна (только градиент) допустимо
	v, err = Validate("#d97757", "")
	if err != nil {
		t.Fatalf("valid cover without pattern: %v", err)
	}
	if v.Color != "#D97757" || v.Pattern != "" {
		t.Fatalf("got %+v", v)
	}

	if _, err := Validate("not-a-color", "star"); !errors.Is(err, ErrInvalidColor) {
		t.Fatalf("want ErrInvalidColor, got %v", err)
	}
	if _, err := Validate("#7C6CF2", "unknown"); !errors.Is(err, ErrUnknownPattern) {
		t.Fatalf("want ErrUnknownPattern, got %v", err)
	}
}

func TestCatalog(t *testing.T) {
	list := Patterns()
	if len(list) == 0 {
		t.Fatal("empty pattern catalog")
	}
	for _, p := range list {
		if !Known(p) {
			t.Fatalf("pattern %q not Known", p)
		}
	}
	if Known("") || Known("missing") {
		t.Fatal("empty or missing pattern should not be Known")
	}
	// изменение копии не портит каталог
	list[0] = "hacked"
	if !Known("star") {
		t.Fatal("Patterns() did not return a copy")
	}
}
