package domain

import "time"

type SendingStatus string

const (
	StatusPending SendingStatus = "pending"
	StatusSending SendingStatus = "sending"
	StatusSent    SendingStatus = "sent"
	StatusFailed  SendingStatus = "failed"
)

const RetryCountLimit = 3

type Message struct {
	ID                   int64         `json:"id" db:"id"`
	Content              string        `json:"content" db:"content"`
	RecipientPhoneNumber string        `json:"recipient_phone_number" db:"recipient_phone_number"`
	SendingStatus        SendingStatus `json:"sending_status" db:"sending_status"`
	RetryCount           int8          `json:"retry_count" db:"retry_count"`
	CreatedAt            time.Time     `json:"created_at" db:"created_at"`
	UpdatedAt            time.Time     `json:"updated_at" db:"updated_at"`
}
