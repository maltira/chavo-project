package utils

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

const (
	BlockEventType  = "block_update"
	StatusEventType = "status_update"
)

type BlockEvent struct {
	EventType string `json:"event_type"`
	BlockerID string `json:"blocker_id"`
	BlockedID string `json:"blocked_id"`
	IsBlocked bool   `json:"is_blocked"`
}

type StatusEvent struct {
	EventType string    `json:"event_type"`
	UserID    string    `json:"user_id"`
	IsOnline  bool      `json:"is_online"`
	LastSeen  time.Time `json:"last_seen"`
}

// PublishBlockEvent публикует событие блокировки в Redis Pub/Sub.
func PublishBlockEvent(rdb *redis.Client, blockerID, blockedID uuid.UUID, isBlocked bool) error {
	payload := BlockEvent{
		EventType: BlockEventType,
		BlockerID: blockerID.String(),
		BlockedID: blockedID.String(),
		IsBlocked: isBlocked,
	}
	bytes, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	return rdb.Publish(context.Background(), "user:block:events", bytes).Err()
}

// PublishStatusEvent публикует событие онлайн-статуса в Redis Pub/Sub.
func PublishStatusEvent(rdb *redis.Client, userID uuid.UUID, online bool, lastSeen time.Time) error {
	event := StatusEvent{
		EventType: StatusEventType,
		UserID:    userID.String(),
		IsOnline:  online,
		LastSeen:  lastSeen,
	}
	bytes, err := json.Marshal(event)
	if err != nil {
		return err
	}
	return rdb.Publish(context.Background(), "user:status:events", bytes).Err()
}
