package dto

import (
	"github.com/google/uuid"

	"github.com/maltira/chavo-project-backend/services/conversation-service/internal/models"
)

type CreateGroupRequest struct {
	Name        string      `json:"name"        binding:"required"`
	Description *string     `json:"description"`
	AvatarURL   *string     `json:"avatar_url"`
	Visibility  string      `json:"visibility"`
	MemberIDs   []uuid.UUID `json:"member_ids"`
}

type UpdateGroupRequest struct {
	Name        *string `json:"name"`
	Description *string `json:"description"`
	AvatarURL   *string `json:"avatar_url"`
	Visibility  *string `json:"visibility"`
}

type AddMembersRequest struct {
	UserIDs []uuid.UUID `json:"user_ids" binding:"required,min=1"`
}

type SetRoleRequest struct {
	Role string `json:"role" binding:"required"`
}

// GroupResponse: invite_token присутствует только в момент создания токена и больше нигде не возвращается.
type GroupResponse struct {
	models.Conversation
	InviteToken *string `json:"invite_token,omitempty"`
}

type MemberListResponse struct {
	Items  []models.Member `json:"items"`
	Limit  int             `json:"limit"`
	Offset int             `json:"offset"`
}

type RequestJoinRequest struct {
	InviteToken string `json:"invite_token" binding:"required"`
}

type InviteLinkResponse struct {
	InviteToken string `json:"invite_token"`
}

type JoinRequestListResponse struct {
	Items  []models.JoinRequest `json:"items"`
	Limit  int                  `json:"limit"`
	Offset int                  `json:"offset"`
}

type PublicGroupListResponse struct {
	Items  []models.PublicGroup `json:"items"`
	Limit  int                  `json:"limit"`
	Offset int                  `json:"offset"`
}
