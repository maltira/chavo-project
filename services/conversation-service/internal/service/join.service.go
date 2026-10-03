package service

import (
	"context"
	"errors"

	"github.com/google/uuid"

	"github.com/maltira/chavo-project-backend/services/conversation-service/internal/apperror"
	"github.com/maltira/chavo-project-backend/services/conversation-service/internal/events"
	"github.com/maltira/chavo-project-backend/services/conversation-service/internal/models"
	"github.com/maltira/chavo-project-backend/services/conversation-service/internal/repository"
)

type JoinService interface {
	InviteInfo(ctx context.Context, userID uuid.UUID, token string) (*models.InviteInfo, error)
	RegenerateInvite(ctx context.Context, adminID, convID uuid.UUID) (string, error)
	RequestJoin(ctx context.Context, userID, convID uuid.UUID, token string) (*models.JoinRequest, error)
	ListRequests(ctx context.Context, adminID, convID uuid.UUID, status string, limit, offset int) ([]models.JoinRequest, error)
	Approve(ctx context.Context, adminID, convID, requestID uuid.UUID) error
	Reject(ctx context.Context, adminID, convID, requestID uuid.UUID) error
}

type joinService struct {
	db       *repository.DB
	convs    repository.ConversationRepository
	requests repository.JoinRequestRepository
	outbox   repository.OutboxRepository
}

func NewJoinService(
	db *repository.DB,
	convs repository.ConversationRepository,
	requests repository.JoinRequestRepository,
	outbox repository.OutboxRepository,
) JoinService {
	return &joinService{db: db, convs: convs, requests: requests, outbox: outbox}
}

func (s *joinService) publish(ctx context.Context, q repository.DBTX, convID uuid.UUID, eventType string, payload any) error {
	return s.outbox.Insert(ctx, q, events.TopicConversationEvents, convID.String(), events.NewEvent(eventType, payload))
}

func (s *joinService) InviteInfo(ctx context.Context, userID uuid.UUID, token string) (*models.InviteInfo, error) {
	q := s.db.Q()
	conv, err := s.convs.FindByInviteHash(ctx, q, hashInviteToken(token))
	if err != nil {
		return nil, err
	}
	count, err := s.convs.CountMembers(ctx, q, conv.ID)
	if err != nil {
		return nil, err
	}
	isMember := true
	if _, err = s.convs.MemberRole(ctx, q, conv.ID, userID); errors.Is(err, apperror.ErrNotFound) {
		isMember = false
	} else if err != nil {
		return nil, err
	}
	pending, err := s.requests.PendingStatus(ctx, q, conv.ID, userID)
	if err != nil {
		return nil, err
	}
	return &models.InviteInfo{
		ConversationID: conv.ID,
		Name:           conv.Name,
		Description:    conv.Description,
		AvatarURL:      conv.AvatarURL,
		MembersCount:   count,
		IsMember:       isMember,
		RequestStatus:  pending,
	}, nil
}

// RegenerateInvite выпускает новый токен для приватной группы; старая ссылка перестаёт работать.
func (s *joinService) RegenerateInvite(ctx context.Context, adminID, convID uuid.UUID) (string, error) {
	var token string
	err := s.db.WithTx(ctx, func(tx repository.DBTX) error {
		conv, err := s.convs.Lock(ctx, tx, convID)
		if err != nil {
			return err
		}
		if err = ensureGroupAdmin(ctx, s.convs, tx, convID, adminID); err != nil {
			return err
		}
		if conv.Visibility != models.VisibilityPrivate {
			return apperror.ErrForbidden
		}
		t, hash, err := newInviteToken()
		if err != nil {
			return err
		}
		token = t
		return s.convs.UpdateGroup(ctx, tx, convID, map[string]any{"invite_token_hash": hash})
	})
	return token, err
}

// RequestJoin создаёт заявку в приватную группу; токен должен принадлежать именно этой группе.
func (s *joinService) RequestJoin(ctx context.Context, userID, convID uuid.UUID, token string) (*models.JoinRequest, error) {
	var req *models.JoinRequest
	err := s.db.WithTx(ctx, func(tx repository.DBTX) error {
		conv, err := s.convs.Lock(ctx, tx, convID)
		if err != nil {
			return err
		}
		if conv.ConversationType != models.ConversationGroup || conv.Visibility != models.VisibilityPrivate {
			return apperror.ErrForbidden
		}
		// Неверный токен неотличим от несуществующей группы.
		byToken, err := s.convs.FindByInviteHash(ctx, tx, hashInviteToken(token))
		if err != nil || byToken.ID != convID {
			return apperror.ErrNotFound
		}

		if _, err = s.convs.MemberRole(ctx, tx, convID, userID); err == nil {
			return apperror.ErrAlreadyMember
		} else if !errors.Is(err, apperror.ErrNotFound) {
			return err
		}

		if req, err = s.requests.Create(ctx, tx, convID, userID); err != nil {
			return err
		}
		adminIDs, err := s.convs.AdminIDs(ctx, tx, convID)
		if err != nil {
			return err
		}
		return s.publish(ctx, tx, convID, events.TypeJoinRequestCreated, events.JoinRequestCreatedPayload{
			ConversationID: convID, RequestID: req.ID, UserID: userID, AdminIDs: adminIDs,
		})
	})
	if err != nil {
		return nil, err
	}
	return req, nil
}

func (s *joinService) ListRequests(ctx context.Context, adminID, convID uuid.UUID, status string, limit, offset int) ([]models.JoinRequest, error) {
	if status == "" {
		status = models.JoinPending
	}
	if status != models.JoinPending && status != models.JoinApproved && status != models.JoinRejected {
		return nil, apperror.ErrIncorrectData
	}
	q := s.db.Q()
	if err := ensureGroupAdmin(ctx, s.convs, q, convID, adminID); err != nil {
		return nil, err
	}
	return s.requests.List(ctx, q, convID, status, limit, offset)
}

func (s *joinService) Approve(ctx context.Context, adminID, convID, requestID uuid.UUID) error {
	return s.resolve(ctx, adminID, convID, requestID, true)
}

func (s *joinService) Reject(ctx context.Context, adminID, convID, requestID uuid.UUID) error {
	return s.resolve(ctx, adminID, convID, requestID, false)
}

func (s *joinService) resolve(ctx context.Context, adminID, convID, requestID uuid.UUID, approve bool) error {
	return s.db.WithTx(ctx, func(tx repository.DBTX) error {
		if _, err := s.convs.Lock(ctx, tx, convID); err != nil {
			return err
		}
		if err := ensureGroupAdmin(ctx, s.convs, tx, convID, adminID); err != nil {
			return err
		}
		req, err := s.requests.GetForUpdate(ctx, tx, requestID)
		if err != nil {
			return err
		}
		if req.ConversationID != convID {
			return apperror.ErrNotFound
		}
		if req.Status != models.JoinPending {
			return apperror.ErrJoinRequestNotPending
		}

		if !approve {
			return s.requests.SetStatus(ctx, tx, requestID, models.JoinRejected)
		}

		if err = s.convs.AddMember(ctx, tx, convID, req.UserID, models.RoleMember); err != nil {
			return err
		}
		if err = s.requests.SetStatus(ctx, tx, requestID, models.JoinApproved); err != nil {
			return err
		}
		memberIDs, err := s.convs.MemberIDs(ctx, tx, convID)
		if err != nil {
			return err
		}
		return s.publish(ctx, tx, convID, events.TypeJoinRequestApproved, events.JoinRequestApprovedPayload{
			ConversationID: convID, RequestID: requestID, UserID: req.UserID, ApprovedBy: adminID, MemberIDs: memberIDs,
		})
	})
}
