package api

import (
	"net/http"

	"github.com/gin-gonic/gin"

	userv1 "github.com/maltira/chavo-project-backend/proto/gen/go/user/v1"
	"github.com/maltira/chavo-project-backend/services/api-gateway/internal/httpx"
)

func (h *Handler) createProfile(c *gin.Context) {
	var body struct {
		Username    string  `json:"username"`
		DisplayName string  `json:"display_name"`
		Bio         *string `json:"bio"`
		AvatarURL   *string `json:"avatar_url"`
	}
	if !httpx.BindJSON(c, &body) {
		return
	}
	if _, err := h.users.CreateProfile(c.Request.Context(), &userv1.CreateProfileRequest{
		Username: body.Username, DisplayName: body.DisplayName, Bio: body.Bio, AvatarUrl: body.AvatarURL,
	}); err != nil {
		h.fail(c, err)
		return
	}
	h.profiles.Created(c.Request.Context(), userID(c))
	httpx.Success(c, http.StatusCreated, "Профиль успешно создан")
}

func (h *Handler) getMe(c *gin.Context) {
	resp, err := h.users.GetMe(c.Request.Context(), &userv1.GetMeRequest{})
	if err != nil {
		h.fail(c, err)
		return
	}
	c.JSON(http.StatusOK, toProfile(resp.GetProfile()))
}

func (h *Handler) updateMe(c *gin.Context) {
	var body struct {
		Username    *string `json:"username"`
		DisplayName *string `json:"display_name"`
		Bio         *string `json:"bio"`
		AvatarURL   *string `json:"avatar_url"`
	}
	if !httpx.BindJSON(c, &body) {
		return
	}
	if _, err := h.users.UpdateMe(c.Request.Context(), &userv1.UpdateMeRequest{
		Username: body.Username, DisplayName: body.DisplayName, Bio: body.Bio, AvatarUrl: body.AvatarURL,
	}); err != nil {
		h.fail(c, err)
		return
	}
	httpx.Success(c, http.StatusOK, "Профиль успешно обновлён")
}

func (h *Handler) searchProfiles(c *gin.Context) {
	resp, err := h.users.SearchProfiles(c.Request.Context(), &userv1.SearchProfilesRequest{
		Query: c.Query("q"), Limit: queryInt(c, "limit"), Offset: queryInt(c, "offset"),
	})
	if err != nil {
		h.fail(c, err)
		return
	}
	c.JSON(http.StatusOK, mapSlice(resp.GetProfiles(), toProfile))
}

func (h *Handler) getProfile(c *gin.Context) {
	resp, err := h.users.GetProfile(c.Request.Context(), &userv1.GetProfileRequest{UserId: c.Param("user_id")})
	if err != nil {
		h.fail(c, err)
		return
	}
	c.JSON(http.StatusOK, toProfile(resp.GetProfile()))
}

func (h *Handler) listBlocked(c *gin.Context) {
	resp, err := h.users.ListBlocked(c.Request.Context(), &userv1.ListBlockedRequest{
		Limit: queryInt(c, "limit"), Offset: queryInt(c, "offset"),
	})
	if err != nil {
		h.fail(c, err)
		return
	}
	c.JSON(http.StatusOK, listJSON[blockedJSON]{
		Items: mapSlice(resp.GetItems(), toBlocked), Limit: resp.GetLimit(), Offset: resp.GetOffset(),
	})
}

func (h *Handler) blockStatus(c *gin.Context) {
	resp, err := h.users.GetBlockStatus(c.Request.Context(), &userv1.GetBlockStatusRequest{UserId: c.Param("user_id")})
	if err != nil {
		h.fail(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"blocked_by_me": resp.GetBlockedByMe(), "blocked_by_them": resp.GetBlockedByThem()})
}

func (h *Handler) block(c *gin.Context) {
	if _, err := h.users.BlockUser(c.Request.Context(), &userv1.BlockUserRequest{UserId: c.Param("user_id")}); err != nil {
		h.fail(c, err)
		return
	}
	httpx.Success(c, http.StatusOK, "Пользователь заблокирован")
}

func (h *Handler) unblock(c *gin.Context) {
	if _, err := h.users.UnblockUser(c.Request.Context(), &userv1.UnblockUserRequest{UserId: c.Param("user_id")}); err != nil {
		h.fail(c, err)
		return
	}
	httpx.Success(c, http.StatusOK, "Пользователь разблокирован")
}

func (h *Handler) getSettings(c *gin.Context) {
	resp, err := h.users.GetSettings(c.Request.Context(), &userv1.GetSettingsRequest{})
	if err != nil {
		h.fail(c, err)
		return
	}
	c.JSON(http.StatusOK, toSettings(resp.GetSettings()))
}

func (h *Handler) updateSettings(c *gin.Context) {
	var body struct {
		AllowGroupInvites *bool `json:"allow_group_invites"`
		ShowOnlineStatus  *bool `json:"show_online_status"`
	}
	if !httpx.BindJSON(c, &body) {
		return
	}
	if _, err := h.users.UpdateSettings(c.Request.Context(), &userv1.UpdateSettingsRequest{
		AllowGroupInvites: body.AllowGroupInvites, ShowOnlineStatus: body.ShowOnlineStatus,
	}); err != nil {
		h.fail(c, err)
		return
	}
	httpx.Success(c, http.StatusOK, "Настройки успешно обновлены")
}
