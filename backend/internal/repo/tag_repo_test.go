package repo

import (
	"strings"
	"testing"
)

// TestNextTagSeqParsing 锁住配码解析：MAX(tag_id) 末段序号 +1。
// 回归背景：曾用 fmt.Sscanf("MT-%*d-%d") 解析，Go fmt 不支持 %*d（bad verb），
// seq 恒为 0 导致永远发出 MT-2026-000001 撞 UNIQUE。
func TestNextTagSeqParsing(t *testing.T) {
	parse := func(tagID string) int {
		last := tagID[strings.LastIndex(tagID, "-")+1:]
		n := 0
		for _, r := range last {
			if r < '0' || r > '9' {
				t.Fatalf("非数字末段: %q", last)
			}
			n = n*10 + int(r-'0')
		}
		return n
	}

	if got := parse("MT-2026-000074") + 1; got != 75 {
		t.Errorf("074 的下一序号 = %d, want 75", got)
	}
	if got := parse("MT-2026-000001") + 1; got != 2 {
		t.Errorf("001 的下一序号 = %d, want 2", got)
	}
	if got := parse("MT-2025-123456") + 1; got != 123457 {
		t.Errorf("123456 的下一序号 = %d, want 123457", got)
	}
}
