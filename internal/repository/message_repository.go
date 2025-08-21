package repository

import (
	"context"
	"insider-message-service/internal/domain"
	"insider-message-service/internal/types"
	"time"

	"github.com/jackc/pgx"
	"github.com/jackc/pgx/v5/pgxpool"
)

type MessageRepository interface {
	// Creates a new message to send
	Create(ctx context.Context, message *domain.Message) error
	UpdateStatus(ctx context.Context, id int64, status domain.SendingStatus) error
	GetByIDs(ctx context.Context, ids []int64) ([]domain.Message, error)
	// Sets the message as "sending" and increments the retry count.
	ClaimMessageForSending(ctx context.Context, messageID int64) (*domain.Message, error)
	// Retrieves messages that are either in the sending or pending status.
	GetProcessableMessages(ctx context.Context, limit int) ([]domain.Message, error)
	// Resets the status of stuck messages to "pending". Also checks retry count.
	ResetProcessableStuckedSendingMessages(ctx context.Context) (int64, error)
}

type messageRepository struct {
	db *pgxpool.Pool
}

func NewMessageRepository(db *pgxpool.Pool) MessageRepository {
	return &messageRepository{db: db}
}

func (r *messageRepository) UpdateStatus(ctx context.Context, id int64, status domain.SendingStatus) error {
	sql := `UPDATE messages 
			SET sending_status = $1, updated_at = $2 
			WHERE id = $3`

	cmdTag, err := r.db.Exec(ctx, sql, status, time.Now(), id)
	if err != nil {
		return err
	}

	if cmdTag.RowsAffected() == 0 {
		return types.ErrNotFound
	}

	return nil
}

// Retrieves messages that are either in the sending or pending status.
func (r *messageRepository) GetProcessableMessages(ctx context.Context, limit int) ([]domain.Message, error) {

	sql := `SELECT 
				id, 
				content, 
				recipient_phone_number, 
				sending_status, 
				retry_count,
				created_at, 
				updated_at 
             FROM messages 
			 WHERE sending_status = $1 AND retry_count < $2
			 ORDER BY id ASC 
			 LIMIT $3`

	rows, err := r.db.Query(ctx, sql,
		domain.StatusPending, domain.RetryCountLimit, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	messages := make([]domain.Message, 0)
	for rows.Next() {
		var msg domain.Message
		if err := rows.Scan(&msg.ID, &msg.Content, &msg.RecipientPhoneNumber, &msg.SendingStatus, &msg.RetryCount, &msg.CreatedAt, &msg.UpdatedAt); err != nil {
			return nil, err
		}
		messages = append(messages, msg)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return messages, nil
}

func (r *messageRepository) GetByIDs(ctx context.Context, ids []int64) ([]domain.Message, error) {
	if len(ids) == 0 {
		return []domain.Message{}, nil
	}

	sql := `SELECT id, content, recipient_phone_number, sending_status, created_at, updated_at 
             FROM messages WHERE id = ANY($1)`

	rows, err := r.db.Query(ctx, sql, ids)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	messagesMap := make(map[int64]domain.Message)
	for rows.Next() {
		var msg domain.Message
		if err := rows.Scan(&msg.ID, &msg.Content, &msg.RecipientPhoneNumber, &msg.SendingStatus, &msg.CreatedAt, &msg.UpdatedAt); err != nil {
			return nil, err
		}
		messagesMap[msg.ID] = msg
	}

	// Redis'ten gelen ID sırasını korumak için sonuçları sırala
	result := make([]domain.Message, len(ids))
	for i, id := range ids {
		result[i] = messagesMap[id]
	}

	return result, rows.Err()
}

func (r *messageRepository) ClaimMessageForSending(ctx context.Context, messageID int64) (*domain.Message, error) {
	sql := `
        UPDATE messages 
        SET sending_status = $1, updated_at = NOW(), retry_count = retry_count + 1
        WHERE id = $2 AND sending_status = $3
        RETURNING id, content, recipient_phone_number, sending_status, retry_count, created_at, updated_at`

	var msg domain.Message
	err := r.db.QueryRow(ctx, sql, domain.StatusSending, messageID, domain.StatusPending).Scan(
		&msg.ID,
		&msg.Content,
		&msg.RecipientPhoneNumber,
		&msg.SendingStatus,
		&msg.RetryCount,
		&msg.CreatedAt,
		&msg.UpdatedAt,
	)

	if err == pgx.ErrNoRows {
		return nil, types.ErrNoRows
	}

	return &msg, err
}

func (r *messageRepository) ResetProcessableStuckedSendingMessages(ctx context.Context) (int64, error) {
	sql := `
        UPDATE messages 
        SET sending_status = $1, updated_at = NOW() 
        WHERE sending_status = $2 AND retry_count < $3`

	cmdTag, err := r.db.Exec(ctx, sql, domain.StatusPending, domain.StatusSending, domain.RetryCountLimit)
	if err != nil {
		return 0, err
	}

	return cmdTag.RowsAffected(), nil
}

func (r *messageRepository) Create(ctx context.Context, message *domain.Message) error {
	sql := `
        INSERT INTO messages (content, recipient_phone_number, sending_status) 
        VALUES ($1, $2, $3) 
        RETURNING id, created_at, updated_at`

	err := r.db.QueryRow(ctx, sql, message.Content, message.RecipientPhoneNumber, message.SendingStatus).Scan(
		&message.ID,
		&message.CreatedAt,
		&message.UpdatedAt,
	)

	return err
}
