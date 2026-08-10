package application

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

type TypingService interface {
	StartTyping(conversationID uint, userType string, userID uint) error
	StopTyping(conversationID uint, userType string, userID uint) error
	GetTypingUsers(conversationID uint) ([]map[string]interface{}, error)
}

type typingService struct {
	redis *redis.Client
}

func NewTypingService(redis *redis.Client) TypingService {
	return &typingService{redis: redis}
}

func (s *typingService) StartTyping(conversationID uint, userType string, userID uint) error {
	ctx := context.Background()
	key := fmt.Sprintf("typing:%d:%s:%d", conversationID, userType, userID)
	return s.redis.Set(ctx, key, "typing", 5*time.Second).Err()
}

func (s *typingService) StopTyping(conversationID uint, userType string, userID uint) error {
	ctx := context.Background()
	key := fmt.Sprintf("typing:%d:%s:%d", conversationID, userType, userID)
	return s.redis.Del(ctx, key).Err()
}

func (s *typingService) GetTypingUsers(conversationID uint) ([]map[string]interface{}, error) {
	ctx := context.Background()
	pattern := fmt.Sprintf("typing:%d:*", conversationID)
	keys, err := s.redis.Keys(ctx, pattern).Result()
	if err != nil {
		return nil, err
	}

	var users []map[string]interface{}
	for _, key := range keys {
		var convID uint
		var userType string
		var userID uint
		fmt.Sscanf(key, "typing:%d:%s:%d", &convID, &userType, &userID)
		users = append(users, map[string]interface{}{
			"conversation_id": convID,
			"user_type":       userType,
			"user_id":         userID,
		})
	}
	return users, nil
}
