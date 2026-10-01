package dto

import "github.com/maltira/chavo-project-backend/services/user-service/internal/models"

type BlockedListResponse struct {
	Items  []models.BlockedEntry `json:"items"`
	Limit  int                   `json:"limit"`
	Offset int                   `json:"offset"`
}

type BlockStatusResponse struct {
	BlockedByMe   bool `json:"blocked_by_me"`
	BlockedByThem bool `json:"blocked_by_them"`
}
