package status

import (
	"errors"
	"strings"
	"testing"
	"time"
)

func TestCatalog(t *testing.T) {
	want := []string{"moon", "shine", "ok", "flame", "block", "heart"}
	got := Icons()
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("icons: %v", got)
	}
	for _, i := range want {
		if !KnownIcon(i) {
			t.Fatalf("%q must be known", i)
		}
	}
	if KnownIcon("harmony") || KnownIcon("") {
		t.Fatal("unknown icon accepted")
	}
	// Копии: изменение результата не портит каталог.
	Icons()[0] = "x"
	if Icons()[0] != "moon" {
		t.Fatal("Icons returned shared slice")
	}
	if len(Durations()) != 5 {
		t.Fatalf("durations: %v", Durations())
	}
}

func TestValidate(t *testing.T) {
	now := time.Date(2026, 10, 10, 12, 0, 0, 0, time.UTC)
	ok := Input{Icon: "shine", Text: "  Сегодня отдыхаю 🌿 ", Background: "#34d399", Foreground: "#064E3B", Duration: "4h"}

	v, err := Validate(ok, now)
	if err != nil || v.Icon != "shine" || v.Text != "Сегодня отдыхаю 🌿" || v.Background != "#34D399" || v.Foreground != "#064E3B" ||
		v.ExpiresAt == nil || !v.ExpiresAt.Equal(now.Add(4*time.Hour)) {
		t.Fatalf("valid: %+v %v", v, err)
	}
	never := ok
	never.Duration = "never"
	if v, err := Validate(never, now); err != nil || v.ExpiresAt != nil {
		t.Fatalf("never: %+v %v", v, err)
	}
	long := ok
	long.Text = strings.Repeat("я", MaxTextLength)
	if _, err := Validate(long, now); err != nil {
		t.Fatalf("max length must pass: %v", err)
	}

	with := func(f func(*Input)) Input { in := ok; f(&in); return in }
	for name, tc := range map[string]struct {
		in   Input
		want error
	}{
		"unknown icon":  {with(func(i *Input) { i.Icon = "harmony" }), ErrUnknownIcon},
		"no icon":       {with(func(i *Input) { i.Icon = "" }), ErrUnknownIcon},
		"bad duration":  {with(func(i *Input) { i.Duration = "2h" }), ErrUnknownDuration},
		"no duration":   {with(func(i *Input) { i.Duration = "" }), ErrUnknownDuration},
		"no background": {with(func(i *Input) { i.Background = "" }), ErrInvalidColor},
		"short color":   {with(func(i *Input) { i.Foreground = "#FFF" }), ErrInvalidColor},
		"no hash":       {with(func(i *Input) { i.Background = "34D399" }), ErrInvalidColor},
		"alpha":         {with(func(i *Input) { i.Background = "#34D399FF" }), ErrInvalidColor},
		"named color":   {with(func(i *Input) { i.Foreground = "red" }), ErrInvalidColor},
		"empty text":    {with(func(i *Input) { i.Text = "" }), ErrInvalidText},
		"blank text":    {with(func(i *Input) { i.Text = "   " }), ErrInvalidText},
		"too long":      {with(func(i *Input) { i.Text = strings.Repeat("я", MaxTextLength+1) }), ErrInvalidText},
		"newline":       {with(func(i *Input) { i.Text = "две\nстроки" }), ErrInvalidText},
		"tab":           {with(func(i *Input) { i.Text = "таб\tвнутри" }), ErrInvalidText},
		"invalid utf-8": {with(func(i *Input) { i.Text = "bad\xffutf8" }), ErrInvalidText},
	} {
		if _, err := Validate(tc.in, now); !errors.Is(err, tc.want) {
			t.Errorf("%s: got %v, want %v", name, err, tc.want)
		}
	}
}
