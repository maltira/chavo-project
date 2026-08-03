package dto

// * Информация

type MessageResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}

// ! Ошибки

type ErrorResponse struct {
	Code  int    `json:"code"`
	Error string `json:"error"`
}
