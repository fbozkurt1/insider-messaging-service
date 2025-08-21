package cache

import (
	"context"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
)

const (
	sentMessagesKey = "sent_messages"
)

type MessageCache interface {
	AddSentMessage(ctx context.Context, messageID int64, sentAt time.Time) error
	GetSentMessageIDs(ctx context.Context, page int, pageSize int) ([]int64, int64, error)
}

type redisMessageCache struct {
	client *redis.Client
}

func NewMessageCache(client *redis.Client) MessageCache {
	return &redisMessageCache{client: client}
}

func (r *redisMessageCache) AddSentMessage(ctx context.Context, messageID int64, sentAt time.Time) error {
	score := float64(sentAt.Unix())
	member := redis.Z{
		Score:  score,
		Member: strconv.FormatInt(messageID, 10),
	}

	err := r.client.ZAdd(ctx, sentMessagesKey, member).Err()
	return err
}

func (r *redisMessageCache) GetSentMessageIDs(ctx context.Context, page int, pageSize int) ([]int64, int64, error) {
	total, err := r.client.ZCard(ctx, sentMessagesKey).Result()
	if err != nil {
		return nil, 0, err
	}

	start := (page - 1) * pageSize
	stop := start + pageSize - 1

	stringIDs, err := r.client.ZRevRange(ctx, sentMessagesKey, int64(start), int64(stop)).Result()
	if err != nil {
		return nil, 0, err
	}

	ids := make([]int64, len(stringIDs))
	for i, sID := range stringIDs {
		id, _ := strconv.ParseInt(sID, 10, 64)
		ids[i] = id
	}

	return ids, total, nil
}
