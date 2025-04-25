package redisdb

import (
	"context"
	"fmt"
	"time"

	"event-pool/blockchain/storage"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

const (
	DefaultTimeout = 5 * time.Second
)

// Factory creates a Redis storage
func Factory(config map[string]interface{}, logger *zap.SugaredLogger) (storage.Storage, error) {
	addr, ok := config["addr"]
	if !ok {
		return nil, fmt.Errorf("addr not found")
	}

	addrStr, ok := addr.(string)
	if !ok {
		return nil, fmt.Errorf("addr is not a string")
	}

	password, _ := config["password"].(string)
	dbNum, _ := config["db"].(int)

	return NewRedisStorage(addrStr, password, dbNum, logger)
}

// NewRedisStorage creates the new storage reference with Redis
func NewRedisStorage(addr string, password string, db int, logger *zap.SugaredLogger) (storage.Storage, error) {
	client := redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: password,
		DB:       db,
	})

	// Check connection
	ctx, cancel := context.WithTimeout(context.Background(), DefaultTimeout)
	defer cancel()

	_, err := client.Ping(ctx).Result()
	if err != nil {
		return nil, err
	}

	kv := &redisKV{client: client}

	return storage.NewKeyValueStorage(logger.Named("redis"), kv), nil
}

// redisKV is the Redis implementation of the kv storage
type redisKV struct {
	client *redis.Client
}

// Set sets the key-value pair in Redis storage
func (r *redisKV) Set(p []byte, v []byte) error {
	ctx, cancel := context.WithTimeout(context.Background(), DefaultTimeout)
	defer cancel()

	return r.client.Set(ctx, string(p), v, 0).Err()
}

// Get retrieves the key-value pair in Redis storage
func (r *redisKV) Get(p []byte) ([]byte, bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), DefaultTimeout)
	defer cancel()

	data, err := r.client.Get(ctx, string(p)).Bytes()
	if err != nil {
		if err == redis.Nil {
			return nil, false, nil
		}
		return nil, false, err
	}

	return data, true, nil
}

// Delete deletes the key in Redis storage
func (r *redisKV) Delete(p []byte) bool {
	ctx, cancel := context.WithTimeout(context.Background(), DefaultTimeout)
	defer cancel()

	result, err := r.client.Del(ctx, string(p)).Result()
	if err != nil {
		return false
	}

	return result > 0
}

// Close closes the Redis storage instance
func (r *redisKV) Close() error {
	return r.client.Close()
}
