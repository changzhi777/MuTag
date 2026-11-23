package api

// 管理员启动播种：ADMIN_SEED="username:password[:显示名]" 且 admins 表为空时创建首个管理员。
// 已有账号时静默跳过（幂等）；密码哈希入库后 env 可移除。

import (
	"context"
	"log"
	"strings"

	"mutag/backend/internal/domain"
	"mutag/backend/internal/repo"
)

// SeedAdmin 播种首个管理员（幂等）。
func SeedAdmin(store *repo.Store, seed string) {
	if seed == "" || store == nil {
		return
	}
	parts := strings.SplitN(seed, ":", 3)
	if len(parts) < 2 || parts[0] == "" || parts[1] == "" {
		log.Printf("[admin-seed] ADMIN_SEED 格式应为 username:password[:显示名]，跳过播种")
		return
	}
	ctx := context.Background()
	n, err := store.Admins().Count(ctx)
	if err != nil {
		log.Printf("[admin-seed] count: %v", err)
		return
	}
	if n > 0 {
		return // 已有管理员，跳过
	}
	hash, err := hashPassword(parts[1])
	if err != nil {
		log.Printf("[admin-seed] hash: %v", err)
		return
	}
	display := ""
	if len(parts) > 2 {
		display = parts[2]
	}
	if _, err := store.Admins().Create(ctx, &domain.Admin{
		Username: parts[0], PasswordHash: hash, DisplayName: display,
	}); err != nil {
		log.Printf("[admin-seed] create %s: %v", parts[0], err)
		return
	}
	log.Printf("[admin-seed] 已创建首个管理员 %s", parts[0])
}
