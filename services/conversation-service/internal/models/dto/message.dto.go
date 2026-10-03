package dto

import (
	"github.com/google/uuid"

	"github.com/maltira/chavo-project-backend/services/conversation-service/internal/models"
)

type SendMessageRequest struct {
	ConversationID   *uuid.UUID `json:"conversation_id"`
	RecipientID      *uuid.UUID `json:"recipient_id"`
	Content          string     `json:"content"             binding:"required"`
	ReplyToMessageID *uuid.UUID `json:"reply_to_message_id"`
}

type EditMessageRequest struct {
	Content string `json:"content" binding:"required"`
}

type MarkReadRequest struct {
	MessageID uuid.UUID `json:"message_id" binding:"required"`
}

type MessageListResponse struct {
	Items      []models.Message `json:"items"`
	Limit      int              `json:"limit"`
	NextBefore *uuid.UUID       `json:"next_before"`
}

type ReadersResponse struct {
	UserIDs []uuid.UUID `json:"user_ids"`
}

type ConversationListResponse struct {
	Items  []models.ConversationSummary `json:"items"`
	Limit  int                          `json:"limit"`
	Offset int                          `json:"offset"`
}
