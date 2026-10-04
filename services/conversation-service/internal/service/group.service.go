package service

import (
	"context"
	"errors"
	"net/url"
	"strings"
	"unicode/utf8"

	"github.com/google/uuid"

	"github.com/maltira/chavo-project-backend/services/conversation-service/internal/apperror"
	"github.com/maltira/chavo-project-backend/services/conversation-service/internal/client/userclient"
	"github.com/maltira/chavo-project-backend/services/conversation-service/internal/events"
	"github.com/maltira/chavo-project-backend/services/conversation-service/internal/models"
	"github.com/maltira/chavo-project-backend/services/conversation-service/internal/repository"
)

const (
	maxGroupNameLength   = 100
	maxDescriptionLength = 255
	maxAvatarURLLength   = 2048
	maxUsersPerRequest   = 100
	maxGroupMembers      = 5000
)

type CreateGroupInput struct {
	Name        string
	Description *string
	AvatarURL   *string
	Visibility  string
	MemberIDs   []uuid.UUID
}

// UpdateGroupInput: nil — поле не меняется; для description и avatar_url пустая строка очищает значение.
type UpdateGroupInput struct {
	Name        *string
	Description *string
	AvatarURL   *string
	Visibility  *string
}

// GroupResult содержит invite-токен только в момент его создания: в БД хранится лишь хэш.
type GroupResult struct {
	Conversation models.Conversation
	InviteToken  *string
}

type GroupService interface {
	Create(ctx context.Context, creatorID uuid.UUID, in CreateGroupInput) (*GroupResult, error)
	Update(ctx context.Context, userID, convID uuid.UUID, in UpdateGroupInput) (*GroupResult, error)
	Delete(ctx context.Context, userID, convID uuid.UUID) error
	Join(ctx context.Context, userID, convID uuid.UUID) error
	ListMembers(ctx context.Context, userID, convID uuid.UUID, limit, offset int) ([]models.Member, error)
	AddMembers(ctx context.Context, adminID, convID uuid.UUID, userIDs []uuid.UUID) error
	SetRole(ctx context.Context, adminID, convID, targetID uuid.UUID, role string) error
	// RemoveMember: kick или выход; ban=true (только admin, не себя) ещё и закрывает возвращение в группу.
	RemoveMember(ctx context.Context, actorID, convID, targetID uuid.UUID, ban bool) error
	ListBans(ctx context.Context, adminID, convID uuid.UUID, limit, offset int) ([]models.Ban, error)
	Unban(ctx context.Context, adminID, convID, targetID uuid.UUID) error
}

type groupService struct {
	db     *repository.DB
	convs  repository.ConversationRepository
	outbox repository.OutboxRepository
	users  userclient.Client
	bans   repository.BanRepository
}

func NewGroupService(
	db *repository.DB,
	convs repository.ConversationRepository,
	outbox repository.OutboxRepository,
	users userclient.Client,
	bans repository.BanRepository,
) GroupService {
	return &groupService{db: db, convs: convs, outbox: outbox, users: users, bans: bans}
}

func (s *groupService) publish(ctx context.Context, q repository.DBTX, topic string, convID uuid.UUID, eventType string, payload any) error {
	return s.outbox.Insert(ctx, q, topic, convID.String(), events.NewEvent(eventType, payload))
}

func validVisibility(v string) bool {
	return v == models.VisibilityPublic || v == models.VisibilityPrivate
}

// optionalText обрезает пробелы и превращает пустую строку в nil.
func optionalText(v *string, maxLen int) (*string, error) {
	if v == nil {
		return nil, nil
	}
	t := strings.TrimSpace(*v)
	if utf8.RuneCountInString(t) > maxLen {
		return nil, apperror.ErrIncorrectData
	}
	if t == "" {
		return nil, nil
	}
	return &t, nil
}

// optionalAvatarURL: как optionalText, но допускает только http(s)-ссылки с host.
func optionalAvatarURL(v *string) (*string, error) {
	t, err := optionalText(v, maxAvatarURLLength)
	if err != nil || t == nil {
		return t, err
	}
	u, err := url.Parse(*t)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return nil, apperror.ErrIncorrectData
	}
	return t, nil
}

// ensureCapacity требует, чтобы строка conversation была заблокирована в этой транзакции:
// иначе параллельные вступления могут вместе превысить лимит.
func ensureCapacity(ctx context.Context, convs repository.ConversationRepository, q repository.DBTX, convID uuid.UUID, adding int) error {
	n, err := convs.CountMembers(ctx, q, convID)
	if err != nil {
		return err
	}
	if n+adding > maxGroupMembers {
		return apperror.ErrGroupFull
	}
	return nil
}

func validateGroupName(name string) (string, error) {
	n := strings.TrimSpace(name)
	if c := utf8.RuneCountInString(n); c < 1 || c > maxGroupNameLength {
		return "", apperror.ErrIncorrectData
	}
	return n, nil
}

// uniqueOthers убирает дубликаты и самого актора, сохраняя порядок.
func uniqueOthers(ids []uuid.UUID, self uuid.UUID) []uuid.UUID {
	seen := map[uuid.UUID]bool{self: true}
	res := make([]uuid.UUID, 0, len(ids))
	for _, id := range ids {
		if !seen[id] {
			seen[id] = true
			res = append(res, id)
		}
	}
	return res
}

// ensureInvitable проверяет в user-service, что пользователи существуют и их можно пригласить в группу:
// приглашения не запрещены и нет блокировки между ними и приглашающим.
func (s *groupService) ensureInvitable(ctx context.Context, inviterID uuid.UUID, ids []uuid.UUID) error {
	for _, id := range ids {
		exists, err := s.users.UserExists(ctx, id)
		if err != nil {
			return err
		}
		if !exists {
			return apperror.ErrUserNotFound
		}
		allowed, err := s.users.GroupInviteAllowed(ctx, id, inviterID)
		if err != nil {
			return err
		}
		if !allowed {
			return apperror.ErrInviteNotAllowed
		}
	}
	return nil
}

// requireGroupAdmin: участник, группа (не direct), роль admin.
func (s *groupService) requireGroupAdmin(ctx context.Context, q repository.DBTX, convID, userID uuid.UUID) error {
	return ensureGroupAdmin(ctx, s.convs, q, convID, userID)
}

func ensureGroupAdmin(ctx context.Context, convs repository.ConversationRepository, q repository.DBTX, convID, userID uuid.UUID) error {
	access, err := convs.Access(ctx, q, convID, userID)
	if err != nil {
		return err
	}
	if access.Type != models.ConversationGroup || access.Role != models.RoleAdmin {
		return apperror.ErrForbidden
	}
	return nil
}

func (s *groupService) Create(ctx context.Context, creatorID uuid.UUID, in CreateGroupInput) (*GroupResult, error) {
	name, err := validateGroupName(in.Name)
	if err != nil {
		return nil, err
	}
	description, err := optionalText(in.Description, maxDescriptionLength)
	if err != nil {
		return nil, err
	}
	avatar, err := optionalAvatarURL(in.AvatarURL)
	if err != nil {
		return nil, err
	}
	visibility := in.Visibility
	if visibility == "" {
		visibility = models.VisibilityPrivate
	}
	if !validVisibility(visibility) {
		return nil, apperror.ErrIncorrectData
	}
	members := uniqueOthers(in.MemberIDs, creatorID)
	if len(members) > maxUsersPerRequest {
		return nil, apperror.ErrIncorrectData
	}
	if 1+len(members) > maxGroupMembers {
		return nil, apperror.ErrGroupFull
	}
	if err = s.ensureInvitable(ctx, creatorID, members); err != nil {
		return nil, err
	}

	res := &GroupResult{}
	var tokenHash *string
	if visibility == models.VisibilityPrivate {
		token, hash, err := newInviteToken()
		if err != nil {
			return nil, err
		}
		res.InviteToken, tokenHash = &token, &hash
	}

	err = s.db.WithTx(ctx, func(tx repository.DBTX) error {
		convID, err := s.convs.CreateGroup(ctx, tx, repository.NewGroup{
			Name: name, Description: description, AvatarURL: avatar,
			Visibility: visibility, InviteTokenHash: tokenHash,
		})
		if err != nil {
			return err
		}
		if err = s.convs.AddMember(ctx, tx, convID, creatorID, models.RoleAdmin); err != nil {
			return err
		}
		for _, id := range members {
			if err = s.convs.AddMember(ctx, tx, convID, id, models.RoleMember); err != nil {
				return err
			}
		}

		conv, err := s.convs.Lock(ctx, tx, convID)
		if err != nil {
			return err
		}
		res.Conversation = *conv

		memberIDs, err := s.convs.MemberIDs(ctx, tx, convID)
		if err != nil {
			return err
		}
		return s.publish(ctx, tx, events.TopicConversationEvents, convID, events.TypeConversationCreated, events.ConversationCreatedPayload{
			ConversationID: convID, ConversationType: models.ConversationGroup, CreatedBy: creatorID, MemberIDs: memberIDs,
		})
	})
	if err != nil {
		return nil, err
	}
	return res, nil
}

func (s *groupService) Update(ctx context.Context, userID, convID uuid.UUID, in UpdateGroupInput) (*GroupResult, error) {
	updates := map[string]any{}
	var fields []string

	if in.Name != nil {
		name, err := validateGroupName(*in.Name)
		if err != nil {
			return nil, err
		}
		updates["name"] = name
		fields = append(fields, "name")
	}
	if in.Description != nil {
		d, err := optionalText(in.Description, maxDescriptionLength)
		if err != nil {
			return nil, err
		}
		updates["description"] = d
		fields = append(fields, "description")
	}
	if in.AvatarURL != nil {
		a, err := optionalAvatarURL(in.AvatarURL)
		if err != nil {
			return nil, err
		}
		updates["avatar_url"] = a
		fields = append(fields, "avatar_url")
	}
	if in.Visibility != nil && !validVisibility(*in.Visibility) {
		return nil, apperror.ErrIncorrectData
	}
	if len(updates) == 0 && in.Visibility == nil {
		return nil, apperror.ErrIncorrectData
	}

	res := &GroupResult{}
	err := s.db.WithTx(ctx, func(tx repository.DBTX) error {
		conv, err := s.convs.Lock(ctx, tx, convID)
		if err != nil {
			return err
		}
		if err = s.requireGroupAdmin(ctx, tx, convID, userID); err != nil {
			return err
		}

		changes := map[string]any{}
		for k, v := range updates {
			changes[k] = v
		}
		changedFields := append([]string(nil), fields...)

		if in.Visibility != nil && *in.Visibility != conv.Visibility {
			changes["visibility"] = *in.Visibility
			changedFields = append(changedFields, "visibility")
			if *in.Visibility == models.VisibilityPrivate {
				token, hash, err := newInviteToken()
				if err != nil {
					return err
				}
				changes["invite_token_hash"] = hash
				res.InviteToken = &token
			} else {
				changes["invite_token_hash"] = nil // публичной группе ссылка не нужна, старые ссылки перестают работать
			}
		}

		if len(changes) > 0 {
			if err = s.convs.UpdateGroup(ctx, tx, convID, changes); err != nil {
				return err
			}
			memberIDs, err := s.convs.MemberIDs(ctx, tx, convID)
			if err != nil {
				return err
			}
			if err = s.publish(ctx, tx, events.TopicConversationEvents, convID, events.TypeConversationUpdated, events.ConversationUpdatedPayload{
				ConversationID: convID, UpdatedBy: userID, Fields: changedFields, MemberIDs: memberIDs,
			}); err != nil {
				return err
			}
		}

		updated, err := s.convs.Lock(ctx, tx, convID)
		if err != nil {
			return err
		}
		res.Conversation = *updated
		return nil
	})
	if err != nil {
		return nil, err
	}
	return res, nil
}

// Delete: участник группы выходит из неё, admin удаляет группу целиком; direct не удаляется.
func (s *groupService) Delete(ctx context.Context, userID, convID uuid.UUID) error {
	return s.db.WithTx(ctx, func(tx repository.DBTX) error {
		if _, err := s.convs.Lock(ctx, tx, convID); err != nil {
			return err
		}
		access, err := s.convs.Access(ctx, tx, convID, userID)
		if err != nil {
			return err
		}
		if access.Type != models.ConversationGroup {
			return apperror.ErrForbidden
		}
		if access.Role == models.RoleMember {
			return s.removeLocked(ctx, tx, convID, userID, userID, false)
		}

		memberIDs, err := s.convs.MemberIDs(ctx, tx, convID)
		if err != nil {
			return err
		}
		if err = s.convs.Delete(ctx, tx, convID); err != nil {
			return err
		}
		return s.publish(ctx, tx, events.TopicConversationEvents, convID, events.TypeConversationDeleted, events.ConversationDeletedPayload{
			ConversationID: convID, DeletedBy: userID, MemberIDs: memberIDs,
		})
	})
}

func (s *groupService) Join(ctx context.Context, userID, convID uuid.UUID) error {
	return s.db.WithTx(ctx, func(tx repository.DBTX) error {
		conv, err := s.convs.Lock(ctx, tx, convID)
		if err != nil {
			return err
		}
		if conv.ConversationType != models.ConversationGroup || conv.Visibility != models.VisibilityPublic {
			return apperror.ErrForbidden
		}
		if _, err = s.convs.MemberRole(ctx, tx, convID, userID); err == nil {
			return apperror.ErrAlreadyMember
		} else if !errors.Is(err, apperror.ErrNotFound) {
			return err
		}
		if banned, err := s.bans.IsBanned(ctx, tx, convID, userID); err != nil {
			return err
		} else if banned {
			return apperror.ErrBanned
		}
		if err = ensureCapacity(ctx, s.convs, tx, convID, 1); err != nil {
			return err
		}
		if err = s.convs.AddMember(ctx, tx, convID, userID, models.RoleMember); err != nil {
			return err
		}
		memberIDs, err := s.convs.MemberIDs(ctx, tx, convID)
		if err != nil {
			return err
		}
		return s.publish(ctx, tx, events.TopicConversationEvents, convID, events.TypeMemberAdded, events.MemberAddedPayload{
			ConversationID: convID, UserIDs: []uuid.UUID{userID}, AddedBy: userID, MemberIDs: memberIDs,
		})
	})
}

func (s *groupService) ListMembers(ctx context.Context, userID, convID uuid.UUID, limit, offset int) ([]models.Member, error) {
	if _, err := s.convs.Access(ctx, s.db.Q(), convID, userID); err != nil {
		return nil, err
	}
	return s.convs.ListMembers(ctx, s.db.Q(), convID, limit, offset)
}

func (s *groupService) AddMembers(ctx context.Context, adminID, convID uuid.UUID, userIDs []uuid.UUID) error {
	ids := uniqueOthers(userIDs, adminID)
	if len(ids) == 0 || len(ids) > maxUsersPerRequest {
		return apperror.ErrIncorrectData
	}
	// Ранний отказ до сетевых вызовов; окончательная проверка прав — в транзакции.
	if err := s.requireGroupAdmin(ctx, s.db.Q(), convID, adminID); err != nil {
		return err
	}
	if err := s.ensureInvitable(ctx, adminID, ids); err != nil {
		return err
	}

	return s.db.WithTx(ctx, func(tx repository.DBTX) error {
		if _, err := s.convs.Lock(ctx, tx, convID); err != nil {
			return err
		}
		if err := s.requireGroupAdmin(ctx, tx, convID, adminID); err != nil {
			return err
		}
		if err := ensureCapacity(ctx, s.convs, tx, convID, len(ids)); err != nil {
			return err
		}
		for _, id := range ids {
			if _, err := s.convs.MemberRole(ctx, tx, convID, id); err == nil {
				return apperror.ErrAlreadyMember
			} else if !errors.Is(err, apperror.ErrNotFound) {
				return err
			}
			if banned, err := s.bans.IsBanned(ctx, tx, convID, id); err != nil {
				return err
			} else if banned {
				return apperror.ErrBanned
			}
			if err := s.convs.AddMember(ctx, tx, convID, id, models.RoleMember); err != nil {
				return err
			}
		}
		memberIDs, err := s.convs.MemberIDs(ctx, tx, convID)
		if err != nil {
			return err
		}
		return s.publish(ctx, tx, events.TopicConversationEvents, convID, events.TypeMemberAdded, events.MemberAddedPayload{
			ConversationID: convID, UserIDs: ids, AddedBy: adminID, MemberIDs: memberIDs,
		})
	})
}

func (s *groupService) SetRole(ctx context.Context, adminID, convID, targetID uuid.UUID, role string) error {
	if role != models.RoleAdmin && role != models.RoleMember {
		return apperror.ErrIncorrectData
	}
	return s.db.WithTx(ctx, func(tx repository.DBTX) error {
		if _, err := s.convs.Lock(ctx, tx, convID); err != nil {
			return err
		}
		if err := s.requireGroupAdmin(ctx, tx, convID, adminID); err != nil {
			return err
		}
		current, err := s.convs.MemberRole(ctx, tx, convID, targetID)
		if err != nil {
			return err
		}
		if current == role {
			return nil
		}
		if current == models.RoleAdmin {
			admins, err := s.convs.CountAdmins(ctx, tx, convID)
			if err != nil {
				return err
			}
			if admins <= 1 {
				return apperror.ErrLastAdmin
			}
		}
		if _, err = s.convs.SetMemberRole(ctx, tx, convID, targetID, role); err != nil {
			return err
		}
		memberIDs, err := s.convs.MemberIDs(ctx, tx, convID)
		if err != nil {
			return err
		}
		return s.publish(ctx, tx, events.TopicConversationEvents, convID, events.TypeConversationUpdated, events.ConversationUpdatedPayload{
			ConversationID: convID, UpdatedBy: adminID, Fields: []string{"member_role"}, TargetUserID: &targetID, MemberIDs: memberIDs,
		})
	})
}

// RemoveMember: kick (admin) или выход (target == actor); ban возможен только при kick.
func (s *groupService) RemoveMember(ctx context.Context, actorID, convID, targetID uuid.UUID, ban bool) error {
	if ban && actorID == targetID {
		return apperror.ErrIncorrectData
	}
	return s.db.WithTx(ctx, func(tx repository.DBTX) error {
		if _, err := s.convs.Lock(ctx, tx, convID); err != nil {
			return err
		}
		return s.removeLocked(ctx, tx, convID, actorID, targetID, ban)
	})
}

func (s *groupService) ListBans(ctx context.Context, adminID, convID uuid.UUID, limit, offset int) ([]models.Ban, error) {
	q := s.db.Q()
	if err := s.requireGroupAdmin(ctx, q, convID, adminID); err != nil {
		return nil, err
	}
	return s.bans.List(ctx, q, convID, limit, offset)
}

func (s *groupService) Unban(ctx context.Context, adminID, convID, targetID uuid.UUID) error {
	return s.db.WithTx(ctx, func(tx repository.DBTX) error {
		if _, err := s.convs.Lock(ctx, tx, convID); err != nil {
			return err
		}
		if err := s.requireGroupAdmin(ctx, tx, convID, adminID); err != nil {
			return err
		}
		removed, err := s.bans.Unban(ctx, tx, convID, targetID)
		if err != nil {
			return err
		}
		if !removed {
			return apperror.ErrNotFound
		}
		return nil
	})
}

// removeLocked требует, чтобы строка conversation уже была заблокирована в этой транзакции.
func (s *groupService) removeLocked(ctx context.Context, tx repository.DBTX, convID, actorID, targetID uuid.UUID, ban bool) error {
	access, err := s.convs.Access(ctx, tx, convID, actorID)
	if err != nil {
		return err
	}
	if access.Type != models.ConversationGroup {
		return apperror.ErrForbidden
	}
	if actorID != targetID && access.Role != models.RoleAdmin {
		return apperror.ErrForbidden
	}

	targetRole, err := s.convs.MemberRole(ctx, tx, convID, targetID)
	if err != nil {
		return err
	}
	if targetRole == models.RoleAdmin {
		admins, err := s.convs.CountAdmins(ctx, tx, convID)
		if err != nil {
			return err
		}
		if admins <= 1 {
			return apperror.ErrLastAdmin
		}
	}
	if _, err = s.convs.RemoveMember(ctx, tx, convID, targetID); err != nil {
		return err
	}
	if ban {
		if err = s.bans.Ban(ctx, tx, convID, targetID, actorID); err != nil {
			return err
		}
	}

	remaining, err := s.convs.MemberIDs(ctx, tx, convID)
	if err != nil {
		return err
	}
	return s.publish(ctx, tx, events.TopicConversationEvents, convID, events.TypeMemberRemoved, events.MemberRemovedPayload{
		ConversationID: convID, UserID: targetID, RemovedBy: actorID, Banned: ban, MemberIDs: append(remaining, targetID),
	})
}
