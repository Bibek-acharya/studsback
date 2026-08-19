package presence

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

type presenceService struct {
	redis *redis.Client
}

func NewPresenceService(redis *redis.Client) PresenceService {
	return &presenceService{redis: redis}
}

func (s *presenceService) SetOnline(userType string, userID uint) error {
	ctx := context.Background()
	key := fmt.Sprintf("presence:%s:%d", userType, userID)
	return s.redis.Set(ctx, key, "online", 30*time.Second).Err()
}

func (s *presenceService) SetOffline(userType string, userID uint) error {
	ctx := context.Background()
	key := fmt.Sprintf("presence:%s:%d", userType, userID)
	return s.redis.Del(ctx, key).Err()
}

func (s *presenceService) IsOnline(userType string, userID uint) (bool, error) {
	ctx := context.Background()
	key := fmt.Sprintf("presence:%s:%d", userType, userID)
	val, err := s.redis.Get(ctx, key).Result()
	if err == redis.Nil {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return val == "online", nil
}

func (s *presenceService) GetOnlineUsers(userType string) ([]uint, error) {
	ctx := context.Background()
	pattern := fmt.Sprintf("presence:%s:*", userType)
	var userIDs []uint
	var cursor uint64
	for {
		keys, nextCursor, err := s.redis.Scan(ctx, cursor, pattern, 100).Result()
		if err != nil {
			return nil, err
		}
		for _, key := range keys {
			var parsedType string
			var id uint
			fmt.Sscanf(key, "presence:%s:%d", &parsedType, &id)
			userIDs = append(userIDs, id)
		}
		cursor = nextCursor
		if cursor == 0 {
			break
		}
	}
	return userIDs, nil
}
