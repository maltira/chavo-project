package api

import (
	"net/http"

	"github.com/gin-gonic/gin"

	conversationv1 "github.com/maltira/chavo-project-backend/proto/gen/go/conversation/v1"
	"github.com/maltira/chavo-project-backend/services/api-gateway/internal/httpx"
)

func (h *Handler) listConversations(c *gin.Context) {
	resp, err := h.conversations.ListConversations(c.Request.Context(), &conversationv1.ListConversationsRequest{
		Limit: queryInt(c, "limit"), Offset: queryInt(c, "offset"),
	})
	if err != nil {
		h.fail(c, err)
		return
	}
	c.JSON(http.StatusOK, listJSON[summaryJSON]{
		Items: mapSlice(resp.GetItems(), toSummary), Limit: resp.GetLimit(), Offset: resp.GetOffset(),
	})
}

func (h *Handler) searchGroups(c *gin.Context) {
	resp, err := h.conversations.SearchPublicGroups(c.Request.Context(), &conversationv1.SearchPublicGroupsRequest{
		Query: c.Query("q"), Limit: queryInt(c, "limit"), Offset: queryInt(c, "offset"),
	})
	if err != nil {
		h.fail(c, err)
		return
	}
	c.JSON(http.StatusOK, listJSON[publicGroupJSON]{
		Items: mapSlice(resp.GetItems(), toPublicGroup), Limit: resp.GetLimit(), Offset: resp.GetOffset(),
	})
}

func (h *Handler) getConversation(c *gin.Context) {
	resp, err := h.conversations.GetConversation(c.Request.Context(), &conversationv1.GetConversationRequest{
		ConversationId: c.Param("id"),
	})
	if err != nil {
		h.fail(c, err)
		return
	}
	c.JSON(http.StatusOK, toSummary(resp.GetConversation()))
}

func (h *Handler) createGroup(c *gin.Context) {
	var body struct {
		Name        string   `json:"name"`
		Description *string  `json:"description"`
		AvatarURL   *string  `json:"avatar_url"`
		Visibility  string   `json:"visibility"`
		MemberIDs   []string `json:"member_ids"`
	}
	if !httpx.BindJSON(c, &body) {
		return
	}
	resp, err := h.conversations.CreateGroup(c.Request.Context(), &conversationv1.CreateGroupRequest{
		Name: body.Name, Description: body.Description, AvatarUrl: body.AvatarURL,
		Visibility: body.Visibility, MemberIds: body.MemberIDs,
	})
	if err != nil {
		h.fail(c, err)
		return
	}
	c.JSON(http.StatusCreated, groupJSON{conversationJSON: toConversation(resp.GetConversation()), InviteToken: resp.InviteToken})
}

func (h *Handler) updateGroup(c *gin.Context) {
	var body struct {
		Name        *string `json:"name"`
		Description *string `json:"description"`
		AvatarURL   *string `json:"avatar_url"`
		Visibility  *string `json:"visibility"`
	}
	if !httpx.BindJSON(c, &body) {
		return
	}
	resp, err := h.conversations.UpdateGroup(c.Request.Context(), &conversationv1.UpdateGroupRequest{
		ConversationId: c.Param("id"),
		Name:           body.Name, Description: body.Description, AvatarUrl: body.AvatarURL, Visibility: body.Visibility,
	})
	if err != nil {
		h.fail(c, err)
		return
	}
	c.JSON(http.StatusOK, groupJSON{conversationJSON: toConversation(resp.GetConversation()), InviteToken: resp.InviteToken})
}

func (h *Handler) deleteConversation(c *gin.Context) {
	if _, err := h.conversations.DeleteConversation(c.Request.Context(), &conversationv1.DeleteConversationRequest{
		ConversationId: c.Param("id"),
	}); err != nil {
		h.fail(c, err)
		return
	}
	httpx.Success(c, http.StatusOK, "Готово")
}

func (h *Handler) joinGroup(c *gin.Context) {
	if _, err := h.conversations.JoinGroup(c.Request.Context(), &conversationv1.JoinGroupRequest{
		ConversationId: c.Param("id"),
	}); err != nil {
		h.fail(c, err)
		return
	}
	httpx.Success(c, http.StatusOK, "Вы вступили в группу")
}

func (h *Handler) listMembers(c *gin.Context) {
	resp, err := h.conversations.ListMembers(c.Request.Context(), &conversationv1.ListMembersRequest{
		ConversationId: c.Param("id"), Limit: queryInt(c, "limit"), Offset: queryInt(c, "offset"),
	})
	if err != nil {
		h.fail(c, err)
		return
	}
	c.JSON(http.StatusOK, listJSON[memberJSON]{
		Items: mapSlice(resp.GetItems(), toMember), Limit: resp.GetLimit(), Offset: resp.GetOffset(),
	})
}

func (h *Handler) addMembers(c *gin.Context) {
	var body struct {
		UserIDs []string `json:"user_ids"`
	}
	if !httpx.BindJSON(c, &body) {
		return
	}
	if _, err := h.conversations.AddMembers(c.Request.Context(), &conversationv1.AddMembersRequest{
		ConversationId: c.Param("id"), UserIds: body.UserIDs,
	}); err != nil {
		h.fail(c, err)
		return
	}
	httpx.Success(c, http.StatusOK, "Участники добавлены")
}

func (h *Handler) setMemberRole(c *gin.Context) {
	var body struct {
		Role string `json:"role"`
	}
	if !httpx.BindJSON(c, &body) {
		return
	}
	if _, err := h.conversations.SetMemberRole(c.Request.Context(), &conversationv1.SetMemberRoleRequest{
		ConversationId: c.Param("id"), UserId: c.Param("user_id"), Role: body.Role,
	}); err != nil {
		h.fail(c, err)
		return
	}
	httpx.Success(c, http.StatusOK, "Роль обновлена")
}

// removeMember: DELETE .../members/me — выход из группы, иначе удаление участника (?ban=true — с баном).
func (h *Handler) removeMember(c *gin.Context) {
	target, leave := c.Param("user_id"), false
	if target == "me" {
		target, leave = userID(c), true
	}
	if _, err := h.conversations.RemoveMember(c.Request.Context(), &conversationv1.RemoveMemberRequest{
		ConversationId: c.Param("id"), UserId: target, Ban: !leave && c.Query("ban") == "true",
	}); err != nil {
		h.fail(c, err)
		return
	}
	if leave {
		httpx.Success(c, http.StatusOK, "Вы вышли из группы")
		return
	}
	httpx.Success(c, http.StatusOK, "Участник удалён")
}

func (h *Handler) listBans(c *gin.Context) {
	resp, err := h.conversations.ListBans(c.Request.Context(), &conversationv1.ListBansRequest{
		ConversationId: c.Param("id"), Limit: queryInt(c, "limit"), Offset: queryInt(c, "offset"),
	})
	if err != nil {
		h.fail(c, err)
		return
	}
	c.JSON(http.StatusOK, listJSON[banJSON]{
		Items: mapSlice(resp.GetItems(), toBan), Limit: resp.GetLimit(), Offset: resp.GetOffset(),
	})
}

func (h *Handler) unban(c *gin.Context) {
	if _, err := h.conversations.Unban(c.Request.Context(), &conversationv1.UnbanRequest{
		ConversationId: c.Param("id"), UserId: c.Param("user_id"),
	}); err != nil {
		h.fail(c, err)
		return
	}
	httpx.Success(c, http.StatusOK, "Пользователь разблокирован")
}

func (h *Handler) inviteInfo(c *gin.Context) {
	resp, err := h.conversations.GetInviteInfo(c.Request.Context(), &conversationv1.GetInviteInfoRequest{Token: c.Param("token")})
	if err != nil {
		h.fail(c, err)
		return
	}
	c.JSON(http.StatusOK, toInviteInfo(resp.GetInfo()))
}

func (h *Handler) regenerateInvite(c *gin.Context) {
	resp, err := h.conversations.RegenerateInvite(c.Request.Context(), &conversationv1.RegenerateInviteRequest{
		ConversationId: c.Param("id"),
	})
	if err != nil {
		h.fail(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"invite_token": resp.GetInviteToken()})
}

func (h *Handler) requestJoin(c *gin.Context) {
	var body struct {
		InviteToken string `json:"invite_token"`
	}
	if !httpx.BindJSON(c, &body) {
		return
	}
	resp, err := h.conversations.RequestJoin(c.Request.Context(), &conversationv1.RequestJoinRequest{
		ConversationId: c.Param("id"), InviteToken: body.InviteToken,
	})
	if err != nil {
		h.fail(c, err)
		return
	}
	c.JSON(http.StatusCreated, toJoinRequest(resp.GetRequest()))
}

func (h *Handler) listJoinRequests(c *gin.Context) {
	resp, err := h.conversations.ListJoinRequests(c.Request.Context(), &conversationv1.ListJoinRequestsRequest{
		ConversationId: c.Param("id"), Status: c.Query("status"),
		Limit: queryInt(c, "limit"), Offset: queryInt(c, "offset"),
	})
	if err != nil {
		h.fail(c, err)
		return
	}
	c.JSON(http.StatusOK, listJSON[joinRequestJSON]{
		Items: mapSlice(resp.GetItems(), toJoinRequest), Limit: resp.GetLimit(), Offset: resp.GetOffset(),
	})
}

func (h *Handler) approveJoinRequest(c *gin.Context) {
	if _, err := h.conversations.ApproveJoinRequest(c.Request.Context(), &conversationv1.ApproveJoinRequestRequest{
		ConversationId: c.Param("id"), RequestId: c.Param("request_id"),
	}); err != nil {
		h.fail(c, err)
		return
	}
	httpx.Success(c, http.StatusOK, "Заявка одобрена")
}

func (h *Handler) rejectJoinRequest(c *gin.Context) {
	if _, err := h.conversations.RejectJoinRequest(c.Request.Context(), &conversationv1.RejectJoinRequestRequest{
		ConversationId: c.Param("id"), RequestId: c.Param("request_id"),
	}); err != nil {
		h.fail(c, err)
		return
	}
	httpx.Success(c, http.StatusOK, "Заявка отклонена")
}

func (h *Handler) markRead(c *gin.Context) {
	var body struct {
		MessageID string `json:"message_id"`
	}
	if !httpx.BindJSON(c, &body) {
		return
	}
	if _, err := h.conversations.MarkRead(c.Request.Context(), &conversationv1.MarkReadRequest{
		ConversationId: c.Param("id"), MessageId: body.MessageID,
	}); err != nil {
		h.fail(c, err)
		return
	}
	httpx.Success(c, http.StatusOK, "Прочитано")
}

func (h *Handler) listReaders(c *gin.Context) {
	resp, err := h.conversations.ListReaders(c.Request.Context(), &conversationv1.ListReadersRequest{
		ConversationId: c.Param("id"), MessageId: c.Param("message_id"),
	})
	if err != nil {
		h.fail(c, err)
		return
	}
	ids := resp.GetUserIds()
	if ids == nil {
		ids = []string{}
	}
	c.JSON(http.StatusOK, gin.H{"user_ids": ids})
}
