package media

import (
	"errors"
	"strings"
	"testing"
)

func TestNewAvatarNameIsValidAndRandom(t *testing.T) {
	a, err := NewAvatarName()
	if err != nil {
		t.Fatal(err)
	}
	b, _ := NewAvatarName()
	if !ValidAvatarName(a) || a == b {
		t.Fatalf("names: %q %q", a, b)
	}
	if got := AvatarURL(a); got != "/media/avatars/"+a {
		t.Fatalf("url: %q", got)
	}
	if got := AvatarKey(a); got != "avatars/"+a {
		t.Fatalf("key: %q", got)
	}
}

func TestAvatarNameFromURL(t *testing.T) {
	name := strings.Repeat("a", 32) + ".jpg"
	if got, ok := AvatarNameFromURL("/media/avatars/" + name); !ok || got != name {
		t.Fatalf("own url: %q %v", got, ok)
	}
	for _, bad := range []string{
		"https://example.com/e2e.png",
		"/media/avatars/../../etc/passwd",
		"/media/avatars/" + strings.Repeat("A", 32) + ".jpg", // только нижний регистр
		"/media/avatars/" + strings.Repeat("a", 32) + ".png",
		"/media/avatars/" + strings.Repeat("a", 31) + ".jpg",
		"/media/other/" + name,
		"/media/avatars/" + name + "?x=1",
		"",
	} {
		if _, ok := AvatarNameFromURL(bad); ok {
			t.Fatalf("must reject %q", bad)
		}
	}
}

func TestNormalizeAvatarURL(t *testing.T) {
	if got, err := NormalizeAvatarURL(""); err != nil || got != nil {
		t.Fatalf("empty clears: %v %v", got, err)
	}
	own := AvatarURL(strings.Repeat("0", 32) + ".jpg")
	if got, err := NormalizeAvatarURL(own); err != nil || *got != own {
		t.Fatalf("own: %v %v", got, err)
	}
	if _, err := NormalizeAvatarURL("https://tracker.example/pixel.gif"); !errors.Is(err, ErrInvalidAvatarURL) {
		t.Fatalf("external: %v", err)
	}
}
