// Package mute - сроки режима «без звука» для чата: единственный источник истины для conversation-service
// (проверка и расчёт) и Gateway (GET /api/customization). В proto срок передаётся строкой-ключом отсюда.
package mute

import (
	"errors"
	"time"
)

var ErrUnknownDuration = errors.New("unknown mute duration")

// Forever - «навсегда»: дата вместо infinity, чтобы клиенты и pgx работали с обычным временем
var Forever = time.Date(9999, 12, 31, 0, 0, 0, 0, time.UTC)

// Duration - срок; Value 0 - навсегда
type Duration struct {
	Key   string
	Value time.Duration
}

var durations = []Duration{
	{Key: "1h", Value: time.Hour},
	{Key: "8h", Value: 8 * time.Hour},
	{Key: "2d", Value: 48 * time.Hour},
	{Key: "forever", Value: 0},
}

// Durations - все сроки в порядке показа
func Durations() []Duration { return append([]Duration(nil), durations...) }

// Until - момент, до которого чат без звука
func Until(now time.Time, key string) (time.Time, error) {
	for _, d := range durations {
		if d.Key == key {
			if d.Value == 0 {
				return Forever, nil
			}
			return now.UTC().Add(d.Value), nil
		}
	}
	return time.Time{}, ErrUnknownDuration
}
