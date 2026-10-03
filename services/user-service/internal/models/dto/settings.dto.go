package dto

type UpdateSettingsRequest struct {
	AllowGroupInvites *bool `json:"allow_group_invites"`
	ShowOnlineStatus  *bool `json:"show_online_status"`
}
