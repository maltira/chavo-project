package dto

type UpdateSettingsRequest struct {
	SystemLanguage    *string `json:"system_language"`
	Theme             *string `json:"theme"`
	AllowGroupInvites *bool   `json:"allow_group_invites"`
	ShowOnlineStatus  *bool   `json:"show_online_status"`
}
