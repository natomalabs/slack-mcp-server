package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/slack-go/slack"
	"go.uber.org/zap"
)

const (
	// CacheTTL is the default TTL for cached data (6 hours)
	CacheTTL = 6 * time.Hour
)

type RedisClient struct {
	client     *redis.Client
	logger     *zap.Logger
	instanceID string
	userID     string
}

func NewRedisClient(logger *zap.Logger, instanceID string, userID string) (*RedisClient, error) {
	addr := os.Getenv("REDIS_ADDR")
	if addr == "" {
		addr = "localhost:6379"
	}

	password := os.Getenv("REDIS_PASSWORD")

	dbStr := os.Getenv("REDIS_DB")
	db := 0
	if dbStr != "" {
		var err error
		db, err = strconv.Atoi(dbStr)
		if err != nil {
			return nil, fmt.Errorf("invalid REDIS_DB value: %v", err)
		}
	}

	rdb := redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: password,
		DB:       db,
	})

	// Test connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := rdb.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("failed to connect to Redis: %v", err)
	}

	logger.Info("Connected to Redis",
		zap.String("addr", addr),
		zap.Int("db", db))

	return &RedisClient{
		client:     rdb,
		logger:     logger,
		instanceID: instanceID,
		userID:     userID,
	}, nil
}

// getSlackKey generates a standardized Redis key with slack: prefix using INSTANCE_ID/USER_ID namespace
func (r *RedisClient) getSlackKey(resource string) string {
	// instanceID represents the unique workspace identifier (enterpriseID for enterprise workspaces, teamID for non-enterprise)
	return fmt.Sprintf("slack:%s/%s:%s", r.instanceID, r.userID, resource)
}

func (r *RedisClient) SetUsers(ctx context.Context, users []slack.User) error {
	data, err := json.Marshal(users)
	if err != nil {
		return fmt.Errorf("failed to marshal users: %v", err)
	}

	key := r.getSlackKey("users")
	err = r.client.Set(ctx, key, data, CacheTTL).Err()
	if err != nil {
		return fmt.Errorf("failed to set users in Redis: %v", err)
	}

	r.logger.Info("Cached users to Redis",
		zap.String("instance_id", r.instanceID),
		zap.String("user_id", r.userID),
		zap.Int("count", len(users)))
	return nil
}

func (r *RedisClient) GetUsers(ctx context.Context) ([]slack.User, error) {
	key := r.getSlackKey("users")
	data, err := r.client.Get(ctx, key).Result()
	if err != nil {
		if err == redis.Nil {
			return nil, nil // No data found
		}
		return nil, fmt.Errorf("failed to get users from Redis: %v", err)
	}

	var users []slack.User
	err = json.Unmarshal([]byte(data), &users)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal users: %v", err)
	}

	r.logger.Info("Loaded users from Redis",
		zap.String("instance_id", r.instanceID),
		zap.String("user_id", r.userID),
		zap.Int("count", len(users)))
	return users, nil
}

func (r *RedisClient) SetChannels(ctx context.Context, channels []Channel) error {
	data, err := json.Marshal(channels)
	if err != nil {
		return fmt.Errorf("failed to marshal channels: %v", err)
	}

	key := r.getSlackKey("channels")
	err = r.client.Set(ctx, key, data, CacheTTL).Err()
	if err != nil {
		return fmt.Errorf("failed to set channels in Redis: %v", err)
	}

	r.logger.Info("Cached channels to Redis",
		zap.String("instance_id", r.instanceID),
		zap.String("user_id", r.userID),
		zap.Int("count", len(channels)))
	return nil
}

func (r *RedisClient) GetChannels(ctx context.Context) ([]Channel, error) {
	key := r.getSlackKey("channels")
	data, err := r.client.Get(ctx, key).Result()
	if err != nil {
		if err == redis.Nil {
			return nil, nil // No data found
		}
		return nil, fmt.Errorf("failed to get channels from Redis: %v", err)
	}

	var channels []Channel
	err = json.Unmarshal([]byte(data), &channels)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal channels: %v", err)
	}

	r.logger.Info("Loaded channels from Redis",
		zap.String("instance_id", r.instanceID),
		zap.String("user_id", r.userID),
		zap.Int("count", len(channels)))
	return channels, nil
}

func (r *RedisClient) Close() error {
	return r.client.Close()
}
