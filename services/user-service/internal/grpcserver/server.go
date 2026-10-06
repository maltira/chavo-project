// Package grpcserver — gRPC-транспорт user-service поверх сервисного слоя.
package grpcserver

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"google.golang.org/protobuf/types/known/timestamppb"

	userv1 "github.com/maltira/chavo-project-backend/proto/gen/go/user/v1"
	"github.com/maltira/chavo-project-backend/proto/grpcx"
	"github.com/maltira/chavo-project-backend/services/user-service/internal/apperror"
	"github.com/maltira/chavo-project-backend/services/user-service/internal/models"
	"github.com/maltira/chavo-project-backend/services/user-service/internal/presence"
	"github.com/maltira/chavo-project-backend/services/user-service/internal/service"
)

type Server struct {
	userv1.UnimplementedUserServiceServer
	userv1.UnimplementedUserInternalServiceServer

	profiles service.ProfileService
	blocks   service.BlockService
	settings service.SettingsService
	presence presence.Checker
}

func New(profiles service.ProfileService, blocks service.BlockService, settings service.SettingsService, presence presence.Checker) *Server {
	return &Server{profiles: profiles, blocks: blocks, settings: settings, presence: presence}
}

// currentUser — пользователь из metadata x-user-id, проставленной Gateway.
func currentUser(ctx context.Context) (uuid.UUID, error) {
	id, err := uuid.Parse(grpcx.IncomingUserID(ctx))
	if err != nil {
		return uuid.Nil, apperror.ErrUnauthorized
	}
	return id, nil
}

func parseID(s string) (uuid.UUID, error) {
	id, err := uuid.Parse(s)
	if err != nil {
		return uuid.Nil, apperror.ErrInvalidUUID
	}
	return id, nil
}

func clamp(v int32, def, max int) int {
	if v < 1 || int(v) > max {
		return def
	}
	return int(v)
}

func ts(t time.Time) *timestamppb.Timestamp { return timestamppb.New(t) }

// profilesToProto: онлайн и last_seen_at видны другим только при show_online_status; self видит всё своё.
func (s *Server) profilesToProto(ctx context.Context, viewer uuid.UUID, items []models.Profile) ([]*userv1.Profile, error) {
	ids := make([]uuid.UUID, len(items))
	for i := range items {
		ids[i] = items[i].UserID
	}
	visible, err := s.settings.ShowOnlineStatus(ctx, ids)
	if err != nil {
		return nil, err
	}
	online, err := s.presence.Online(ctx, ids)
	if err != nil {
		return nil, err
	}

	res := make([]*userv1.Profile, len(items))
	for i := range items {
		p := &items[i]
		show := p.UserID == viewer || visible[p.UserID]
		out := &userv1.Profile{
			UserId:      p.UserID.String(),
			Username:    p.Username,
			DisplayName: p.DisplayName,
			Bio:         p.Bio,
			AvatarUrl:   p.AvatarURL,
			CreatedAt:   ts(p.CreatedAt),
			UpdatedAt:   ts(p.UpdatedAt),
		}
		if show {
			out.Online = online[p.UserID]
			out.LastSeenAt = ts(p.LastSeenAt)
		}
		res[i] = out
	}
	return res, nil
}

func (s *Server) oneProfile(ctx context.Context, viewer uuid.UUID, p *models.Profile) (*userv1.Profile, error) {
	list, err := s.profilesToProto(ctx, viewer, []models.Profile{*p})
	if err != nil {
		return nil, err
	}
	return list[0], nil
}

func (s *Server) CreateProfile(ctx context.Context, req *userv1.CreateProfileRequest) (*userv1.CreateProfileResponse, error) {
	userID, err := currentUser(ctx)
	if err != nil {
		return nil, err
	}
	if req.GetUsername() == "" || req.GetDisplayName() == "" {
		return nil, apperror.ErrIncorrectData
	}
	err = s.profiles.Create(ctx, userID, service.CreateProfileInput{
		Username:    req.GetUsername(),
		DisplayName: req.GetDisplayName(),
		Bio:         req.Bio,
		AvatarURL:   req.AvatarUrl,
	})
	if err != nil {
		return nil, err
	}
	return &userv1.CreateProfileResponse{}, nil
}

func (s *Server) GetMe(ctx context.Context, _ *userv1.GetMeRequest) (*userv1.GetMeResponse, error) {
	userID, err := currentUser(ctx)
	if err != nil {
		return nil, err
	}
	p, err := s.profiles.FindByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	out, err := s.oneProfile(ctx, userID, p)
	if err != nil {
		return nil, err
	}
	return &userv1.GetMeResponse{Profile: out}, nil
}

func (s *Server) UpdateMe(ctx context.Context, req *userv1.UpdateMeRequest) (*userv1.UpdateMeResponse, error) {
	userID, err := currentUser(ctx)
	if err != nil {
		return nil, err
	}
	data := make(map[string]string)
	if req.Username != nil {
		data["username"] = req.GetUsername()
	}
	if req.DisplayName != nil {
		data["display_name"] = req.GetDisplayName()
	}
	if req.Bio != nil {
		data["bio"] = req.GetBio()
	}
	if req.AvatarUrl != nil {
		data["avatar_url"] = req.GetAvatarUrl()
	}
	if err = s.profiles.Update(ctx, userID, data); err != nil {
		return nil, err
	}
	return &userv1.UpdateMeResponse{}, nil
}

func (s *Server) GetProfile(ctx context.Context, req *userv1.GetProfileRequest) (*userv1.GetProfileResponse, error) {
	viewer, err := currentUser(ctx)
	if err != nil {
		return nil, err
	}
	id, err := parseID(req.GetUserId())
	if err != nil {
		return nil, err
	}
	p, err := s.profiles.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	out, err := s.oneProfile(ctx, viewer, p)
	if err != nil {
		return nil, err
	}
	return &userv1.GetProfileResponse{Profile: out}, nil
}

func (s *Server) SearchProfiles(ctx context.Context, req *userv1.SearchProfilesRequest) (*userv1.SearchProfilesResponse, error) {
	viewer, err := currentUser(ctx)
	if err != nil {
		return nil, err
	}
	if req.GetQuery() == "" {
		return nil, apperror.ErrIncorrectData
	}
	offset := int(req.GetOffset())
	if offset < 0 {
		offset = 0
	}
	items, err := s.profiles.GetAllBySearch(ctx, req.GetQuery(), clamp(req.GetLimit(), 8, 20), offset)
	if err != nil {
		return nil, err
	}
	out, err := s.profilesToProto(ctx, viewer, items)
	if err != nil {
		return nil, err
	}
	return &userv1.SearchProfilesResponse{Profiles: out}, nil
}

func (s *Server) ListBlocked(ctx context.Context, req *userv1.ListBlockedRequest) (*userv1.ListBlockedResponse, error) {
	userID, err := currentUser(ctx)
	if err != nil {
		return nil, err
	}
	limit := clamp(req.GetLimit(), 20, 100)
	offset := int(req.GetOffset())
	if offset < 0 {
		offset = 0
	}
	items, err := s.blocks.GetBlockedUsers(ctx, userID, limit, offset)
	if err != nil {
		return nil, err
	}
	out := make([]*userv1.BlockedEntry, len(items))
	for i, b := range items {
		out[i] = &userv1.BlockedEntry{
			BlockedUserId: b.BlockedUserID.String(),
			Username:      b.Username,
			DisplayName:   b.DisplayName,
			AvatarUrl:     b.AvatarURL,
			BlockedAt:     ts(b.BlockedAt),
		}
	}
	return &userv1.ListBlockedResponse{Items: out, Limit: int32(limit), Offset: int32(offset)}, nil
}

func (s *Server) GetBlockStatus(ctx context.Context, req *userv1.GetBlockStatusRequest) (*userv1.GetBlockStatusResponse, error) {
	userID, err := currentUser(ctx)
	if err != nil {
		return nil, err
	}
	target, err := parseID(req.GetUserId())
	if err != nil {
		return nil, err
	}
	byMe, byThem, err := s.blocks.GetBlockStatus(ctx, userID, target)
	if err != nil {
		return nil, err
	}
	return &userv1.GetBlockStatusResponse{BlockedByMe: byMe, BlockedByThem: byThem}, nil
}

func (s *Server) BlockUser(ctx context.Context, req *userv1.BlockUserRequest) (*userv1.BlockUserResponse, error) {
	userID, err := currentUser(ctx)
	if err != nil {
		return nil, err
	}
	target, err := parseID(req.GetUserId())
	if err != nil {
		return nil, err
	}
	if err = s.blocks.BlockUser(ctx, userID, target); err != nil {
		return nil, err
	}
	return &userv1.BlockUserResponse{}, nil
}

func (s *Server) UnblockUser(ctx context.Context, req *userv1.UnblockUserRequest) (*userv1.UnblockUserResponse, error) {
	userID, err := currentUser(ctx)
	if err != nil {
		return nil, err
	}
	target, err := parseID(req.GetUserId())
	if err != nil {
		return nil, err
	}
	if err = s.blocks.UnblockUser(ctx, userID, target); err != nil {
		return nil, err
	}
	return &userv1.UnblockUserResponse{}, nil
}

func (s *Server) GetSettings(ctx context.Context, _ *userv1.GetSettingsRequest) (*userv1.GetSettingsResponse, error) {
	userID, err := currentUser(ctx)
	if err != nil {
		return nil, err
	}
	st, err := s.settings.GetSettings(ctx, userID)
	if err != nil {
		return nil, err
	}
	return &userv1.GetSettingsResponse{Settings: &userv1.Settings{
		UserId:            st.UserID.String(),
		AllowGroupInvites: st.AllowGroupInvites,
		ShowOnlineStatus:  st.ShowOnlineStatus,
		CreatedAt:         ts(st.CreatedAt),
		UpdatedAt:         ts(st.UpdatedAt),
	}}, nil
}

func (s *Server) UpdateSettings(ctx context.Context, req *userv1.UpdateSettingsRequest) (*userv1.UpdateSettingsResponse, error) {
	userID, err := currentUser(ctx)
	if err != nil {
		return nil, err
	}
	data := make(map[string]any)
	if req.AllowGroupInvites != nil {
		data["allow_group_invites"] = req.GetAllowGroupInvites()
	}
	if req.ShowOnlineStatus != nil {
		data["show_online_status"] = req.GetShowOnlineStatus()
	}
	if err = s.settings.UpdateSettings(ctx, userID, data); err != nil {
		return nil, err
	}
	return &userv1.UpdateSettingsResponse{}, nil
}

// --- internal ---

func (s *Server) MessagingAllowed(ctx context.Context, req *userv1.MessagingAllowedRequest) (*userv1.MessagingAllowedResponse, error) {
	sender, err := parseID(req.GetSenderId())
	if err != nil {
		return nil, err
	}
	recipient, err := parseID(req.GetRecipientId())
	if err != nil {
		return nil, err
	}
	bySender, byRecipient, err := s.blocks.GetBlockStatus(ctx, sender, recipient)
	if err != nil {
		return nil, err
	}
	return &userv1.MessagingAllowedResponse{
		Allowed:            !bySender && !byRecipient,
		BlockedBySender:    bySender,
		BlockedByRecipient: byRecipient,
	}, nil
}

func (s *Server) UserExists(ctx context.Context, req *userv1.UserExistsRequest) (*userv1.UserExistsResponse, error) {
	id, err := parseID(req.GetUserId())
	if err != nil {
		return nil, err
	}
	_, err = s.profiles.FindByID(ctx, id)
	switch {
	case err == nil:
		return &userv1.UserExistsResponse{Exists: true}, nil
	case errors.Is(err, apperror.ErrNotFound):
		return &userv1.UserExistsResponse{Exists: false}, nil
	default:
		return nil, err
	}
}

// GroupInviteAllowed: запрещено настройкой пользователя или блокировкой между ним и inviter (в любую сторону).
func (s *Server) GroupInviteAllowed(ctx context.Context, req *userv1.GroupInviteAllowedRequest) (*userv1.GroupInviteAllowedResponse, error) {
	userID, err := parseID(req.GetUserId())
	if err != nil {
		return nil, err
	}
	inviterID, err := parseID(req.GetInviterId())
	if err != nil {
		return nil, err
	}
	st, err := s.settings.GetSettings(ctx, userID)
	if err != nil {
		return nil, err
	}
	allowed := st.AllowGroupInvites
	if allowed {
		byInviter, byInvitee, err := s.blocks.GetBlockStatus(ctx, inviterID, userID)
		if err != nil {
			return nil, err
		}
		allowed = !byInviter && !byInvitee
	}
	return &userv1.GroupInviteAllowedResponse{Allowed: allowed}, nil
}

func (s *Server) PresenceVisible(ctx context.Context, req *userv1.PresenceVisibleRequest) (*userv1.PresenceVisibleResponse, error) {
	if len(req.GetUserIds()) > 1000 {
		return nil, apperror.ErrIncorrectData
	}
	ids := make([]uuid.UUID, 0, len(req.GetUserIds()))
	for _, raw := range req.GetUserIds() {
		id, err := parseID(raw)
		if err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	visible, err := s.settings.ShowOnlineStatus(ctx, ids)
	if err != nil {
		return nil, err
	}
	out := make(map[string]bool, len(visible))
	for id, v := range visible {
		out[id.String()] = v
	}
	return &userv1.PresenceVisibleResponse{Visible: out}, nil
}
