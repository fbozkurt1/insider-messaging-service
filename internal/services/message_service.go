package services

import (
	"context"
	"fmt"
	"insider-message-service/internal/cache"
	"insider-message-service/internal/domain"
	"insider-message-service/internal/repository"
	"insider-message-service/internal/types"
	"log"
	"sync"
	"sync/atomic"
	"time"
)

type MessageService interface {
	GetSentMessages(ctx context.Context, page int, pageSize int) ([]domain.Message, int64, error)
	// Sends all pending messages with batch processing
	SendAllPendingMessages(ctx context.Context) error
	// Sends pending messages with a limit
	SendPendingMessagesWithLimit(ctx context.Context, limit int32) (int32, int32, error)
	// Saves a new message to the repository to send
	CreateMessage(ctx context.Context, content, recipientPhone string) (*domain.Message, error)
}

type messageService struct {
	repo  repository.MessageRepository
	cache cache.MessageCache
}

func NewMessageService(repo repository.MessageRepository, cache cache.MessageCache) MessageService {
	return &messageService{repo: repo, cache: cache}
}

func (s *messageService) CreateMessage(ctx context.Context, content, recipientPhone string) (*domain.Message, error) {

	message := &domain.Message{
		Content:              content,
		RecipientPhoneNumber: recipientPhone,
		SendingStatus:        domain.StatusPending,
	}

	err := s.repo.Create(ctx, message)
	if err != nil {
		return nil, fmt.Errorf("unexpected error occurred while saving message to repository: %w", err)
	}

	return message, nil
}

// sends all pending messages
func (s *messageService) SendAllPendingMessages(ctx context.Context) error {
	// Reset stuck messages before sending
	stuckedCount, _ := s.repo.ResetProcessableStuckedSendingMessages(ctx)
	if stuckedCount > 0 {
		log.Printf("Reset %d stuck messages\n", stuckedCount)
	}

	var batchSize int32 = 100
	var totalProcessed int32 = 0
	for {
		processedInBatch, processableCount, err := s.SendPendingMessagesWithLimit(ctx, batchSize)
		if err != nil {
			return err
		}

		atomic.AddInt32(&totalProcessed, processedInBatch)

		// if processable message count is less than batchSize, break the loop
		if processableCount < batchSize {
			break
		}
	}

	log.Printf("Total processed messages: %d\n", totalProcessed)
	return nil
}

// sends pending messages with a limit
func (s *messageService) SendPendingMessagesWithLimit(ctx context.Context, limit int32) (int32, int32, error) {
	// Reset stuck messages before sending
	stuckedCount, _ := s.repo.ResetProcessableStuckedSendingMessages(ctx)
	if stuckedCount > 0 {
		log.Printf("Reset %d stuck messages\n", stuckedCount)
	}

	pendingMessages, err := s.repo.GetProcessableMessages(ctx, int(limit))
	if err != nil {
		return 0, 0, err
	}

	if len(pendingMessages) == 0 {
		return 0, 0, nil
	}

	processedInBatch := s.sendMessagesBatchConcurrently(ctx, pendingMessages)
	return processedInBatch, int32(len(pendingMessages)), nil
}

// retrieves sent messages from the cache.
func (s *messageService) GetSentMessages(ctx context.Context, page int, pageSize int) ([]domain.Message, int64, error) {
	if page < 1 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 20
	}

	if pageSize > 100 {
		pageSize = 100
	}

	ids, total, err := s.cache.GetSentMessageIDs(ctx, page, pageSize)
	if err != nil {
		// I don't want to hit the database if we have a cache miss
		// because it can be expensive to query with pagination the database.
		// If it is so important, retrying the cache access could be an option.
		return nil, 0, err
	}

	if len(ids) == 0 {
		return []domain.Message{}, total, nil
	}

	// if we need only IDs in the response, also we don't need to query database.
	// We can return directly from cache
	messages, err := s.repo.GetByIDs(ctx, ids)
	if err != nil {
		return nil, 0, err
	}

	return messages, total, nil
}

// sends messages in a batch concurrently. 10 workers are used
func (s *messageService) sendMessagesBatchConcurrently(ctx context.Context, batch []domain.Message) int32 {
	numMessages := len(batch)
	if numMessages == 0 {
		return 0
	}

	messageJobs := make(chan domain.Message, numMessages)

	var processedCount int32

	var wg sync.WaitGroup

	numWorkers := min(numMessages, 10)

	for w := 1; w <= numWorkers; w++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			for message := range messageJobs {
				log.Printf("Worker %d, messageJobs %d is processing...", workerID, message.ID)
				if err := s.sendMessage(ctx, message.ID); err != nil {
					log.Printf("Worker %d failed to process message ID %d: %v", workerID, message.ID, err)
				} else {
					atomic.AddInt32(&processedCount, 1)
				}
			}
		}(w)
	}

	for _, msg := range batch {
		messageJobs <- msg
	}
	close(messageJobs)

	wg.Wait()

	return atomic.LoadInt32(&processedCount)
}

// sends a message and cache it to redis
func (s *messageService) sendMessage(ctx context.Context, messageID int64) error {
	msg, err := s.repo.ClaimMessageForSending(ctx, messageID)
	if err != nil {
		if err == types.ErrNoRows {
			// this message has already been sent
			return nil
		}
		// If there is another database error, return it.
		return err
	}

	log.Printf("Message ID %d is being sent -> Recipient: %s, Content: '%s'\n", msg.ID, msg.RecipientPhoneNumber, msg.Content)
	time.Sleep(250 * time.Millisecond)

	sentSuccessfully := true
	sentDate := time.Now()
	var finalStatus domain.SendingStatus

	// simulate sending message
	if sentSuccessfully {
		finalStatus = domain.StatusSent
		log.Printf("Message ID %d has been sent successfully.\n", msg.ID)
	} else {
		// If failed, could be implemented with a retry mechanism like outbox Pattern etc.
		finalStatus = domain.StatusFailed
		log.Printf("Message ID %d could not be sent.\n", msg.ID)
	}

	if err := s.repo.UpdateStatus(ctx, msg.ID, finalStatus); err != nil {
		return err
	}

	if finalStatus == domain.StatusSent {
		go s.cacheSentMessage(ctx, msg.ID, sentDate)
	}

	return nil
}

// caches the sent message ID and its sent date.
func (s *messageService) cacheSentMessage(ctx context.Context, messageID int64, sentDate time.Time) {
	if err := s.cache.AddSentMessage(ctx, messageID, sentDate); err != nil {
		log.Printf("Failed to add sent message to cache: ID: %d, Error: %v\n", messageID, err)
	}
}
