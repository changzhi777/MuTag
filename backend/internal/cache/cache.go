// Package cache 进程内 TTL 缓存（活码分流用）。
// M1 开发形态：SQLite + 内存缓存（零外部依赖）；上线形态为 Redis（见 deploy/compose），
// 两者语义对齐：Get/Set + 60s TTL。
package cache

import (
	"sync"
	"time"
)

// TTLCache 简单并发安全 TTL 缓存。
type TTLCache struct {
	mu      sync.RWMutex
	data    map[string]entry
	ttl     time.Duration
	nowFunc func() time.Time
}

type entry struct {
	value     string
	expiresAt time.Time
}

// New 构造指定 TTL 的缓存。
func New(ttl time.Duration) *TTLCache {
	return &TTLCache{
		data:    make(map[string]entry),
		ttl:     ttl,
		nowFunc: time.Now,
	}
}

// Get 命中返回值；过期视为未命中并清理。
func (c *TTLCache) Get(key string) (string, bool) {
	c.mu.RLock()
	e, ok := c.data[key]
	c.mu.RUnlock()
	if !ok {
		return "", false
	}
	if c.nowFunc().After(e.expiresAt) {
		c.mu.Lock()
		delete(c.data, key)
		c.mu.Unlock()
		return "", false
	}
	return e.value, true
}

// Set 写入并刷新过期时间。
func (c *TTLCache) Set(key, value string) {
	c.mu.Lock()
	c.data[key] = entry{value: value, expiresAt: c.nowFunc().Add(c.ttl)}
	c.mu.Unlock()
}
