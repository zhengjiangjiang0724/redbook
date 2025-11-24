package auth

import (
	"context"
	"fmt"
	"time"

	"github.com/go-redis/redis/v8"
)

var ctx = context.Background()

// SessionManager persists refresh tokens and blacklist markers in Redis.
type SessionManager struct {
	rdb *redis.Client
}

func NewSessionManager(rdb *redis.Client) *SessionManager {
	return &SessionManager{rdb: rdb}
}

// SaveRefreshToken stores the refresh token for the specific user/device pair.
func (s *SessionManager) SaveRefreshToken(userID uint, device, token string, ttl time.Duration) error {
	key := fmt.Sprintf("rb:session:%d:%s", userID, device)
	return s.rdb.Set(ctx, key, token, ttl).Err()
}

// GetRefreshToken fetches the latest refresh token for a user/device.
func (s *SessionManager) GetRefreshToken(userID uint, device string) (string, error) {
	key := fmt.Sprintf("rb:session:%d:%s", userID, device)
	return s.rdb.Get(ctx, key).Result()
}

// DeleteRefreshToken removes the stored refresh token, used during logout.
func (s *SessionManager) DeleteRefreshToken(userID uint, device string) error {
	key := fmt.Sprintf("rb:session:%d:%s", userID, device)
	return s.rdb.Del(ctx, key).Err()
}

// AddBlackList blacklists a token for the remainder of its lifetime.
func (s *SessionManager) AddBlackList(token string, ttl time.Duration) error {
	key := fmt.Sprintf("rb:black:%s", token)
	return s.rdb.Set(ctx, key, "1", ttl).Err()
}

// InBlackList reports whether a token has been invalidated previously.
func (s *SessionManager) InBlackList(token string) (bool, error) {
	key := fmt.Sprintf("rb:black:%s", token)
	res, err := s.rdb.Exists(ctx, key).Result()
	return res == 1, err
}
