package service

import (
	"context"
	"errors"

	"github.com/google/uuid"

	"github.com/maltira/chavo-project-backend/proto/media"
	"github.com/maltira/chavo-project-backend/services/conversation-service/internal/apperror"
	"github.com/maltira/chavo-project-backend/services/conversation-service/internal/events"
	"github.com/maltira/chavo-project-backend/services/conversation-service/internal/models"
	"github.com/maltira/chavo-project-backend/services/conversation-service/internal/repository"
)

func (s *groupService) ListAvatars(ctx context.Context, userID, convID uuid.UUID) ([]repository.AvatarPhoto, error) {
	q := s.db.Q()
	access, err := s.convs.Access(ctx, q, convID, userID)
	switch {
	case errors.Is(err, apperror.ErrNotMember):
		// фото открытой группы видно в поиске всем - история тоже
		public, perr := s.convs.IsPublicGroup(ctx, q, convID)
		if perr != nil {
			return nil, perr
		}
		if !public {
			return nil, apperror.ErrNotMember
		}
	case err != nil:
		return nil, err
	case access.Type != models.ConversationGroup:
		return nil, apperror.ErrForbidden
	}
	return s.avatars.List(ctx, q, convID)
}

// changeAvatars выполняет изменение истории под блокировкой группы и сообщает участникам, что группа изменилась
func (s *groupService) changeAvatars(ctx context.Context, adminID, convID uuid.UUID, change func(tx repository.DBTX) error) error {
	return s.db.WithTx(ctx, func(tx repository.DBTX) error {
		if _, err := s.convs.Lock(ctx, tx, convID); err != nil {
			return err
		}
		if err := s.requireGroupAdmin(ctx, tx, convID, adminID); err != nil {
			return err
		}
		if err := change(tx); err != nil {
			return err
		}
		memberIDs, err := s.convs.MemberIDs(ctx, tx, convID)
		if err != nil {
			return err
		}
		return s.publish(ctx, tx, events.TopicConversationEvents, convID, events.TypeConversationUpdated, events.ConversationUpdatedPayload{
			ConversationID: convID, UpdatedBy: adminID, Fields: []string{"avatar_url"}, MemberIDs: memberIDs,
		})
	})
}

func (s *groupService) AddAvatar(ctx context.Context, adminID, convID uuid.UUID, url string) ([]string, error) {
	norm, err := media.NormalizeAvatarURL(url)
	if err != nil {
		return nil, err
	}
	// пустой адрес - не фото: убирать аватарку нужно через DeleteAvatar
	if norm == nil {
		return nil, apperror.ErrIncorrectData
	}
	var evicted []string
	err = s.changeAvatars(ctx, adminID, convID, func(tx repository.DBTX) (err error) {
		evicted, err = s.avatars.Add(ctx, tx, convID, *norm, media.MaxAvatarHistory)
		return err
	})
	if err != nil {
		return nil, err
	}
	return evicted, nil
}

func (s *groupService) SelectAvatar(ctx context.Context, adminID, convID, photoID uuid.UUID) error {
	return s.changeAvatars(ctx, adminID, convID, func(tx repository.DBTX) error {
		return s.avatars.Select(ctx, tx, convID, photoID)
	})
}

func (s *groupService) DeleteAvatar(ctx context.Context, adminID, convID uuid.UUID, photoID *uuid.UUID) (string, error) {
	var removed string
	err := s.changeAvatars(ctx, adminID, convID, func(tx repository.DBTX) (err error) {
		removed, err = s.avatars.Delete(ctx, tx, convID, photoID)
		return err
	})
	if err != nil {
		return "", err
	}
	return removed, nil
}
