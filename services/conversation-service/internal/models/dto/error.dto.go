package dto

type MessageResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}

type ErrorResponse struct {
	Code   int    `json:"code"`
	Error  string `json:"error"`
	Reason string `json:"reason,omitempty"`
}
