package mute

import (
	"errors"
	"testing"
	"time"
)

func TestUntil(t *testing.T) {
	now := time.Date(2026, 10, 21, 12, 0, 0, 0, time.UTC)
	cases := map[string]time.Time{
		"1h":      now.Add(time.Hour),
		"8h":      now.Add(8 * time.Hour),
		"2d":      now.Add(48 * time.Hour),
		"forever": Forever,
	}
	for key, want := range cases {
		got, err := Until(now, key)
		if err != nil || !got.Equal(want) {
			t.Errorf("Until(%q) = %v, %v; want %v", key, got, err, want)
		}
	}
	if _, err := Until(now, "3h"); !errors.Is(err, ErrUnknownDuration) {
		t.Errorf("unknown key: err = %v", err)
	}
}

func TestDurationsUnique(t *testing.T) {
	seen := map[string]bool{}
	for _, d := range Durations() {
		if seen[d.Key] {
			t.Fatalf("duplicate key %q", d.Key)
		}
		seen[d.Key] = true
	}
}
