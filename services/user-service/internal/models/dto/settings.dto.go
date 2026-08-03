package dto

type SettingsUpdateRequest struct {
	ShowOnlineStatus *bool   `json:"show_online_status"`
	ShowBirthDate    *string `json:"show_birth_date"`
}
