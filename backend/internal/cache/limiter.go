// limiter.go 进程内令牌桶限流（纪律 4：扫码限流防爬）。
// 开发形态按 ClientIP 维度（活码扫码匿名无 openid）；上线形态换 Redis 令牌桶时，
// 仅需替换本文件实现（Allow 语义不变），调用方零改动。
package cache

import (
	"sync"
	"time"
)

// Limiter 令牌桶限流器：容量控制突发，rate 控制持续速率。
type Limiter struct {
	mu       sync.Mutex
	buckets  map[string]*limiterBucket
	capacity float64 // 桶容量（突发上限）
	rate     float64 // 每秒回填令牌数
	nowFunc  func() time.Time
}

type limiterBucket struct {
	tokens float64
	last   time.Time
}

// NewLimiter 构造限流器；capacity<=0 或 rate<=0 返回 nil（关闭限流）。
func NewLimiter(capacity, rate float64) *Limiter {
	if capacity <= 0 || rate <= 0 {
		return nil
	}
	return &Limiter{
		buckets:  make(map[string]*limiterBucket),
		capacity: capacity,
		rate:     rate,
		nowFunc:  time.Now,
	}
}

// Allow 取一枚令牌；桶空返回 false（调用方回 429）。
// 惰性建桶 + 桶数超 4 倍容量时全扫清理闲置桶（防长尾 IP 撑爆内存）。
func (l *Limiter) Allow(key string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := l.nowFunc()
	if len(l.buckets) > int(l.capacity)*4+1000 {
		for k, b := range l.buckets {
			if now.Sub(b.last) > time.Hour {
				delete(l.buckets, k)
			}
		}
	}

	b, ok := l.buckets[key]
	if !ok {
		b = &limiterBucket{tokens: l.capacity - 1, last: now} // 首次消耗一枚
		l.buckets[key] = b
		return true
	}

	b.tokens += now.Sub(b.last).Seconds() * l.rate
	if b.tokens > l.capacity {
		b.tokens = l.capacity
	}
	b.last = now

	if b.tokens < 1 {
		return false
	}
	b.tokens--
	return true
}
