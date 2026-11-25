package models

import (
	"time"

	"github.com/google/uuid"
)

// AvatarPhoto - фото из истории аватарок; текущее - самое новое
type AvatarPhoto struct {
	ID        uuid.UUID
	URL       string
	CreatedAt time.Time
	IsCurrent bool
}
