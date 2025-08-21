package dto

type MessageResponse struct {
	ID                   int64  `json:"id" db:"id"`
	Content              string `json:"content" db:"content"`
	RecipientPhoneNumber string `json:"recipient_phone_number" db:"recipient_phone_number"`
}

type MessagesResponse struct {
	Messages []MessageResponse `json:"messages"`
	Total    int64             `json:"total"`
}

type MessageJobResponse struct {
	Status string `json:"status"`
}

type CreateMessageRequest struct {
	Content              string `json:"content" binding:"required,max=255"`
	RecipientPhoneNumber string `json:"recipientPhoneNumber" binding:"required,e164"`
}

type ErrorResponse struct {
	Error string `json:"error"`
}
