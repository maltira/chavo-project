package utils

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

// StatusManager управляет online-статусом пользователей.
type StatusManager struct {
	rdb            *redis.Client
	log            *zap.Logger
	updateLastSeen func(ctx context.Context, userID uuid.UUID, lastSeen time.Time) error
}

// NewStatusManager создаёт StatusManager с явными зависимостями.
func NewStatusManager(rdb *redis.Client, log *zap.Logger, updateLastSeen func(context.Context, uuid.UUID, time.Time) error) *StatusManager {
	return &StatusManager{
		rdb:            rdb,
		log:            log,
		updateLastSeen: updateLastSeen,
	}
}

func (m *StatusManager) SetOnline(userID uuid.UUID) {
	if err := PublishStatusEvent(m.rdb, userID, true, time.Now()); err != nil {
		m.log.Error("Failed to publish online event", zap.String("user_id", userID.String()), zap.Error(err))
	}
}

func (m *StatusManager) SetOffline(userID uuid.UUID) {
	t := time.Now()
	if err := PublishStatusEvent(m.rdb, userID, false, t); err != nil {
		m.log.Error("Failed to publish offline event", zap.String("user_id", userID.String()), zap.Error(err))
	}
	if m.updateLastSeen != nil {
		if err := m.updateLastSeen(context.Background(), userID, t); err != nil {
			m.log.Error("Failed to update last_seen", zap.String("user_id", userID.String()), zap.Error(err))
		}
	}
}
