package cache

// 令牌桶限流单测：突发消耗、速率回填、按 key 隔离、关闭态。

import (
	"testing"
	"time"
)

func TestLimiterBurstAndRefill(t *testing.T) {
	l := NewLimiter(3, 5) // 容量 3，每秒回 5 个
	if l == nil {
		t.Fatal("合法参数不应返回 nil")
	}

	// 突发 3 次通过，第 4 次拒绝
	for i := 0; i < 3; i++ {
		if !l.Allow("ip1") {
			t.Fatalf("第 %d 次应通过（桶容量内）", i+1)
		}
	}
	if l.Allow("ip1") {
		t.Fatal("桶空应拒绝")
	}

	// 推进 0.4s（回填 5*0.4=2 个）→ 可再通过 2 次
	l.nowFunc = func() time.Time { return time.Now().Add(400 * time.Millisecond) }
	if !l.Allow("ip1") || !l.Allow("ip1") {
		t.Fatal("回填后应通过 2 次")
	}
	if l.Allow("ip1") {
		t.Fatal("回填耗尽后应拒绝")
	}
}

func TestLimiterKeyIsolation(t *testing.T) {
	l := NewLimiter(1, 1)
	if !l.Allow("a") || !l.Allow("b") {
		t.Fatal("不同 IP 各自独立桶")
	}
	if l.Allow("a") || l.Allow("b") {
		t.Fatal("各自桶空应拒绝")
	}
}

func TestLimiterDisabled(t *testing.T) {
	if NewLimiter(0, 5) != nil || NewLimiter(10, 0) != nil || NewLimiter(-1, 1) != nil {
		t.Fatal("非法参数应返回 nil（关闭限流）")
	}
}
