// Package grpcserver — gRPC-транспорт conversation-service поверх сервисного слоя.
package grpcserver

import (
	"context"

	conversationv1 "github.com/maltira/chavo-project-backend/proto/gen/go/conversation/v1"
	"github.com/maltira/chavo-project-backend/services/conversation-service/internal/service"
)

type Server struct {
	conversationv1.UnimplementedConversationServiceServer
	conversationv1.UnimplementedConversationInternalServiceServer

	convs  service.ConversationService
	groups service.GroupService
	joins  service.JoinService
	msgs   service.MessageService
}

func New(convs service.ConversationService, groups service.GroupService, joins service.JoinService, msgs service.MessageService) *Server {
	return &Server{convs: convs, groups: groups, joins: joins, msgs: msgs}
}

func (s *Server) ListConversations(ctx context.Context, req *conversationv1.ListConversationsRequest) (*conversationv1.ListConversationsResponse, error) {
	user, err := currentUser(ctx)
	if err != nil {
		return nil, err
	}
	limit, offset := page(req.GetLimit(), req.GetOffset(), 30, 100)
	items, err := s.convs.List(ctx, user, limit, offset)
	if err != nil {
		return nil, err
	}
	out := make([]*conversationv1.ConversationSummary, len(items))
	for i := range items {
		out[i] = toSummary(&items[i])
	}
	return &conversationv1.ListConversationsResponse{Items: out, Limit: int32(limit), Offset: int32(offset)}, nil
}

func (s *Server) GetConversation(ctx context.Context, req *conversationv1.GetConversationRequest) (*conversationv1.GetConversationResponse, error) {
	user, err := currentUser(ctx)
	if err != nil {
		return nil, err
	}
	convID, err := parseID(req.GetConversationId())
	if err != nil {
		return nil, err
	}
	item, err := s.convs.Get(ctx, user, convID)
	if err != nil {
		return nil, err
	}
	return &conversationv1.GetConversationResponse{Conversation: toSummary(item)}, nil
}

func (s *Server) SearchPublicGroups(ctx context.Context, req *conversationv1.SearchPublicGroupsRequest) (*conversationv1.SearchPublicGroupsResponse, error) {
	user, err := currentUser(ctx)
	if err != nil {
		return nil, err
	}
	limit, offset := page(req.GetLimit(), req.GetOffset(), 8, 100)
	items, err := s.convs.SearchPublicGroups(ctx, user, req.GetQuery(), limit, offset)
	if err != nil {
		return nil, err
	}
	out := make([]*conversationv1.PublicGroup, len(items))
	for i, g := range items {
		out[i] = &conversationv1.PublicGroup{
			Id:           g.ID.String(),
			Name:         g.Name,
			Description:  g.Description,
			AvatarUrl:    g.AvatarURL,
			MembersCount: int32(g.MembersCount),
			IsMember:     g.IsMember,
		}
	}
	return &conversationv1.SearchPublicGroupsResponse{Items: out, Limit: int32(limit), Offset: int32(offset)}, nil
}

func (s *Server) CreateGroup(ctx context.Context, req *conversationv1.CreateGroupRequest) (*conversationv1.CreateGroupResponse, error) {
	user, err := currentUser(ctx)
	if err != nil {
		return nil, err
	}
	members, err := parseIDs(req.GetMemberIds())
	if err != nil {
		return nil, err
	}
	res, err := s.groups.Create(ctx, user, service.CreateGroupInput{
		Name:        req.GetName(),
		Description: req.Description,
		AvatarURL:   req.AvatarUrl,
		Visibility:  req.GetVisibility(),
		MemberIDs:   members,
	})
	if err != nil {
		return nil, err
	}
	return &conversationv1.CreateGroupResponse{Conversation: toConversation(&res.Conversation), InviteToken: res.InviteToken}, nil
}

func (s *Server) UpdateGroup(ctx context.Context, req *conversationv1.UpdateGroupRequest) (*conversationv1.UpdateGroupResponse, error) {
	user, err := currentUser(ctx)
	if err != nil {
		return nil, err
	}
	convID, err := parseID(req.GetConversationId())
	if err != nil {
		return nil, err
	}
	res, err := s.groups.Update(ctx, user, convID, service.UpdateGroupInput{
		Name:        req.Name,
		Description: req.Description,
		AvatarURL:   req.AvatarUrl,
		Visibility:  req.Visibility,
	})
	if err != nil {
		return nil, err
	}
	return &conversationv1.UpdateGroupResponse{Conversation: toConversation(&res.Conversation), InviteToken: res.InviteToken}, nil
}

func (s *Server) DeleteConversation(ctx context.Context, req *conversationv1.DeleteConversationRequest) (*conversationv1.DeleteConversationResponse, error) {
	user, err := currentUser(ctx)
	if err != nil {
		return nil, err
	}
	convID, err := parseID(req.GetConversationId())
	if err != nil {
		return nil, err
	}
	if err = s.groups.Delete(ctx, user, convID); err != nil {
		return nil, err
	}
	return &conversationv1.DeleteConversationResponse{}, nil
}

func (s *Server) JoinGroup(ctx context.Context, req *conversationv1.JoinGroupRequest) (*conversationv1.JoinGroupResponse, error) {
	user, err := currentUser(ctx)
	if err != nil {
		return nil, err
	}
	convID, err := parseID(req.GetConversationId())
	if err != nil {
		return nil, err
	}
	if err = s.groups.Join(ctx, user, convID); err != nil {
		return nil, err
	}
	return &conversationv1.JoinGroupResponse{}, nil
}

func (s *Server) ListMembers(ctx context.Context, req *conversationv1.ListMembersRequest) (*conversationv1.ListMembersResponse, error) {
	user, err := currentUser(ctx)
	if err != nil {
		return nil, err
	}
	convID, err := parseID(req.GetConversationId())
	if err != nil {
		return nil, err
	}
	limit, offset := page(req.GetLimit(), req.GetOffset(), 50, 200)
	items, err := s.groups.ListMembers(ctx, user, convID, limit, offset)
	if err != nil {
		return nil, err
	}
	out := make([]*conversationv1.Member, len(items))
	for i := range items {
		out[i] = toMember(&items[i])
	}
	return &conversationv1.ListMembersResponse{Items: out, Limit: int32(limit), Offset: int32(offset)}, nil
}

func (s *Server) AddMembers(ctx context.Context, req *conversationv1.AddMembersRequest) (*conversationv1.AddMembersResponse, error) {
	user, err := currentUser(ctx)
	if err != nil {
		return nil, err
	}
	convID, err := parseID(req.GetConversationId())
	if err != nil {
		return nil, err
	}
	ids, err := parseIDs(req.GetUserIds())
	if err != nil {
		return nil, err
	}
	if err = s.groups.AddMembers(ctx, user, convID, ids); err != nil {
		return nil, err
	}
	return &conversationv1.AddMembersResponse{}, nil
}

func (s *Server) SetMemberRole(ctx context.Context, req *conversationv1.SetMemberRoleRequest) (*conversationv1.SetMemberRoleResponse, error) {
	user, err := currentUser(ctx)
	if err != nil {
		return nil, err
	}
	convID, err := parseID(req.GetConversationId())
	if err != nil {
		return nil, err
	}
	target, err := parseID(req.GetUserId())
	if err != nil {
		return nil, err
	}
	if err = s.groups.SetRole(ctx, user, convID, target, req.GetRole()); err != nil {
		return nil, err
	}
	return &conversationv1.SetMemberRoleResponse{}, nil
}

func (s *Server) RemoveMember(ctx context.Context, req *conversationv1.RemoveMemberRequest) (*conversationv1.RemoveMemberResponse, error) {
	user, err := currentUser(ctx)
	if err != nil {
		return nil, err
	}
	convID, err := parseID(req.GetConversationId())
	if err != nil {
		return nil, err
	}
	target, err := parseID(req.GetUserId())
	if err != nil {
		return nil, err
	}
	if err = s.groups.RemoveMember(ctx, user, convID, target, req.GetBan()); err != nil {
		return nil, err
	}
	return &conversationv1.RemoveMemberResponse{}, nil
}

func (s *Server) ListBans(ctx context.Context, req *conversationv1.ListBansRequest) (*conversationv1.ListBansResponse, error) {
	user, err := currentUser(ctx)
	if err != nil {
		return nil, err
	}
	convID, err := parseID(req.GetConversationId())
	if err != nil {
		return nil, err
	}
	limit, offset := page(req.GetLimit(), req.GetOffset(), 50, 200)
	items, err := s.groups.ListBans(ctx, user, convID, limit, offset)
	if err != nil {
		return nil, err
	}
	out := make([]*conversationv1.Ban, len(items))
	for i, b := range items {
		out[i] = &conversationv1.Ban{UserId: b.UserID.String(), BannedBy: b.BannedBy.String(), CreatedAt: ts(b.CreatedAt)}
	}
	return &conversationv1.ListBansResponse{Items: out, Limit: int32(limit), Offset: int32(offset)}, nil
}

func (s *Server) Unban(ctx context.Context, req *conversationv1.UnbanRequest) (*conversationv1.UnbanResponse, error) {
	user, err := currentUser(ctx)
	if err != nil {
		return nil, err
	}
	convID, err := parseID(req.GetConversationId())
	if err != nil {
		return nil, err
	}
	target, err := parseID(req.GetUserId())
	if err != nil {
		return nil, err
	}
	if err = s.groups.Unban(ctx, user, convID, target); err != nil {
		return nil, err
	}
	return &conversationv1.UnbanResponse{}, nil
}

func (s *Server) GetInviteInfo(ctx context.Context, req *conversationv1.GetInviteInfoRequest) (*conversationv1.GetInviteInfoResponse, error) {
	user, err := currentUser(ctx)
	if err != nil {
		return nil, err
	}
	info, err := s.joins.InviteInfo(ctx, user, req.GetToken())
	if err != nil {
		return nil, err
	}
	return &conversationv1.GetInviteInfoResponse{Info: &conversationv1.InviteInfo{
		ConversationId: info.ConversationID.String(),
		Name:           info.Name,
		Description:    info.Description,
		AvatarUrl:      info.AvatarURL,
		MembersCount:   int32(info.MembersCount),
		IsMember:       info.IsMember,
		RequestStatus:  info.RequestStatus,
	}}, nil
}

func (s *Server) RegenerateInvite(ctx context.Context, req *conversationv1.RegenerateInviteRequest) (*conversationv1.RegenerateInviteResponse, error) {
	user, err := currentUser(ctx)
	if err != nil {
		return nil, err
	}
	convID, err := parseID(req.GetConversationId())
	if err != nil {
		return nil, err
	}
	token, err := s.joins.RegenerateInvite(ctx, user, convID)
	if err != nil {
		return nil, err
	}
	return &conversationv1.RegenerateInviteResponse{InviteToken: token}, nil
}

func (s *Server) RequestJoin(ctx context.Context, req *conversationv1.RequestJoinRequest) (*conversationv1.RequestJoinResponse, error) {
	user, err := currentUser(ctx)
	if err != nil {
		return nil, err
	}
	convID, err := parseID(req.GetConversationId())
	if err != nil {
		return nil, err
	}
	jr, err := s.joins.RequestJoin(ctx, user, convID, req.GetInviteToken())
	if err != nil {
		return nil, err
	}
	return &conversationv1.RequestJoinResponse{Request: toJoinRequest(jr)}, nil
}

func (s *Server) ListJoinRequests(ctx context.Context, req *conversationv1.ListJoinRequestsRequest) (*conversationv1.ListJoinRequestsResponse, error) {
	user, err := currentUser(ctx)
	if err != nil {
		return nil, err
	}
	convID, err := parseID(req.GetConversationId())
	if err != nil {
		return nil, err
	}
	limit, offset := page(req.GetLimit(), req.GetOffset(), 50, 200)
	items, err := s.joins.ListRequests(ctx, user, convID, req.GetStatus(), limit, offset)
	if err != nil {
		return nil, err
	}
	out := make([]*conversationv1.JoinRequest, len(items))
	for i := range items {
		out[i] = toJoinRequest(&items[i])
	}
	return &conversationv1.ListJoinRequestsResponse{Items: out, Limit: int32(limit), Offset: int32(offset)}, nil
}

func (s *Server) resolveRequest(ctx context.Context, rawConv, rawReq string, approve bool) error {
	user, err := currentUser(ctx)
	if err != nil {
		return err
	}
	convID, err := parseID(rawConv)
	if err != nil {
		return err
	}
	requestID, err := parseID(rawReq)
	if err != nil {
		return err
	}
	if approve {
		return s.joins.Approve(ctx, user, convID, requestID)
	}
	return s.joins.Reject(ctx, user, convID, requestID)
}

func (s *Server) ApproveJoinRequest(ctx context.Context, req *conversationv1.ApproveJoinRequestRequest) (*conversationv1.ApproveJoinRequestResponse, error) {
	if err := s.resolveRequest(ctx, req.GetConversationId(), req.GetRequestId(), true); err != nil {
		return nil, err
	}
	return &conversationv1.ApproveJoinRequestResponse{}, nil
}

func (s *Server) RejectJoinRequest(ctx context.Context, req *conversationv1.RejectJoinRequestRequest) (*conversationv1.RejectJoinRequestResponse, error) {
	if err := s.resolveRequest(ctx, req.GetConversationId(), req.GetRequestId(), false); err != nil {
		return nil, err
	}
	return &conversationv1.RejectJoinRequestResponse{}, nil
}

func (s *Server) SendMessage(ctx context.Context, req *conversationv1.SendMessageRequest) (*conversationv1.SendMessageResponse, error) {
	user, err := currentUser(ctx)
	if err != nil {
		return nil, err
	}
	convID, err := parseOptionalID(req.ConversationId)
	if err != nil {
		return nil, err
	}
	recipient, err := parseOptionalID(req.RecipientId)
	if err != nil {
		return nil, err
	}
	reply, err := parseOptionalID(req.ReplyToMessageId)
	if err != nil {
		return nil, err
	}
	msg, err := s.msgs.Send(ctx, user, service.SendMessageInput{
		ConversationID:   convID,
		RecipientID:      recipient,
		Content:          req.GetContent(),
		ReplyToMessageID: reply,
	})
	if err != nil {
		return nil, err
	}
	return &conversationv1.SendMessageResponse{Message: toMessage(msg)}, nil
}

func (s *Server) GetMessage(ctx context.Context, req *conversationv1.GetMessageRequest) (*conversationv1.GetMessageResponse, error) {
	user, err := currentUser(ctx)
	if err != nil {
		return nil, err
	}
	msgID, err := parseID(req.GetMessageId())
	if err != nil {
		return nil, err
	}
	msg, err := s.msgs.Get(ctx, user, msgID)
	if err != nil {
		return nil, err
	}
	return &conversationv1.GetMessageResponse{Message: toMessage(msg)}, nil
}

func (s *Server) ListMessages(ctx context.Context, req *conversationv1.ListMessagesRequest) (*conversationv1.ListMessagesResponse, error) {
	user, err := currentUser(ctx)
	if err != nil {
		return nil, err
	}
	convID, err := parseID(req.GetConversationId())
	if err != nil {
		return nil, err
	}
	before, err := parseOptionalID(req.Before)
	if err != nil {
		return nil, err
	}
	limit, _ := page(req.GetLimit(), 0, 50, 100)
	items, err := s.msgs.List(ctx, user, convID, before, limit)
	if err != nil {
		return nil, err
	}
	out := make([]*conversationv1.Message, len(items))
	for i := range items {
		out[i] = toMessage(&items[i])
	}
	resp := &conversationv1.ListMessagesResponse{Items: out, Limit: int32(limit)}
	if len(items) == limit {
		resp.NextBefore = optStr(&items[len(items)-1].ID)
	}
	return resp, nil
}

func (s *Server) EditMessage(ctx context.Context, req *conversationv1.EditMessageRequest) (*conversationv1.EditMessageResponse, error) {
	user, err := currentUser(ctx)
	if err != nil {
		return nil, err
	}
	msgID, err := parseID(req.GetMessageId())
	if err != nil {
		return nil, err
	}
	msg, err := s.msgs.Edit(ctx, user, msgID, req.GetContent())
	if err != nil {
		return nil, err
	}
	return &conversationv1.EditMessageResponse{Message: toMessage(msg)}, nil
}

func (s *Server) DeleteMessage(ctx context.Context, req *conversationv1.DeleteMessageRequest) (*conversationv1.DeleteMessageResponse, error) {
	user, err := currentUser(ctx)
	if err != nil {
		return nil, err
	}
	msgID, err := parseID(req.GetMessageId())
	if err != nil {
		return nil, err
	}
	if err = s.msgs.Delete(ctx, user, msgID); err != nil {
		return nil, err
	}
	return &conversationv1.DeleteMessageResponse{}, nil
}

func (s *Server) MarkRead(ctx context.Context, req *conversationv1.MarkReadRequest) (*conversationv1.MarkReadResponse, error) {
	user, err := currentUser(ctx)
	if err != nil {
		return nil, err
	}
	convID, err := parseID(req.GetConversationId())
	if err != nil {
		return nil, err
	}
	msgID, err := parseID(req.GetMessageId())
	if err != nil {
		return nil, err
	}
	if err = s.msgs.MarkRead(ctx, user, convID, msgID); err != nil {
		return nil, err
	}
	return &conversationv1.MarkReadResponse{}, nil
}

func (s *Server) ListReaders(ctx context.Context, req *conversationv1.ListReadersRequest) (*conversationv1.ListReadersResponse, error) {
	user, err := currentUser(ctx)
	if err != nil {
		return nil, err
	}
	convID, err := parseID(req.GetConversationId())
	if err != nil {
		return nil, err
	}
	msgID, err := parseID(req.GetMessageId())
	if err != nil {
		return nil, err
	}
	ids, err := s.msgs.Readers(ctx, user, convID, msgID)
	if err != nil {
		return nil, err
	}
	return &conversationv1.ListReadersResponse{UserIds: strs(ids)}, nil
}

// --- internal ---

func (s *Server) ListDirectPeers(ctx context.Context, req *conversationv1.ListDirectPeersRequest) (*conversationv1.ListDirectPeersResponse, error) {
	user, err := parseID(req.GetUserId())
	if err != nil {
		return nil, err
	}
	ids, err := s.convs.DirectPeers(ctx, user)
	if err != nil {
		return nil, err
	}
	return &conversationv1.ListDirectPeersResponse{UserIds: strs(ids)}, nil
}
