package dto

type MessagingAllowedResponse struct {
	Allowed            bool `json:"allowed"`
	BlockedBySender    bool `json:"blocked_by_sender"`
	BlockedByRecipient bool `json:"blocked_by_recipient"`
}

type UserExistsResponse struct {
	Exists bool `json:"exists"`
}

type GroupInviteAllowedResponse struct {
	Allowed bool `json:"allowed"`
}
