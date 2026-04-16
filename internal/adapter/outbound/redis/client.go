package redis

import (
	"context"
	"fmt"
	"strings"

	goredis "github.com/redis/go-redis/v9"

	"github.com/enolalab/alfred/internal/config"
)

type Client struct {
	raw       *goredis.Client
	keyPrefix string
}

func NewClient(cfg config.RedisStorageConfig) (*Client, error) {
	if strings.TrimSpace(cfg.Addr) == "" {
		return nil, fmt.Errorf("redis addr is required")
	}
	if strings.TrimSpace(cfg.KeyPrefix) == "" {
		return nil, fmt.Errorf("redis key_prefix is required")
	}

	client := goredis.NewClient(&goredis.Options{
		Addr:     cfg.Addr,
		Username: cfg.Username,
		Password: cfg.Password,
		DB:       cfg.DB,
	})

	return &Client{
		raw:       client,
		keyPrefix: cfg.KeyPrefix,
	}, nil
}

func (c *Client) Ping(ctx context.Context) error {
	if err := c.raw.Ping(ctx).Err(); err != nil {
		return fmt.Errorf("ping redis: %w", err)
	}
	return nil
}

func (c *Client) Close() error {
	if c == nil || c.raw == nil {
		return nil
	}
	return c.raw.Close()
}

func (c *Client) conversationKey(id string) string {
	return c.keyPrefix + ":conversation:" + id
}

func (c *Client) incidentKey(conversationID string) string {
	return c.keyPrefix + ":incident:" + conversationID
}

func (c *Client) dedupeKey(key string) string {
	return c.keyPrefix + ":dedupe:" + key
}
