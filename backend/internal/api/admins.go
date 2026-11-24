// 管理员管理 API（0007）：列表 / 新增 / 改密码与显示名 / 删除（不能删自己）。
package api

import (
	"strconv"

	"github.com/gin-gonic/gin"

	"mutag/backend/internal/domain"
)

// listAdmins GET /api/admin/admins。
func (d Deps) listAdmins(c *gin.Context) {
	items, err := d.Store.DB.QueryContext(c.Request.Context(),
		`SELECT id, username, display_name, created_at FROM admins ORDER BY id`)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	defer items.Close()

	list := []gin.H{}
	for items.Next() {
		var id int64
		var username, displayName, createdAt string
		if err := items.Scan(&id, &username, &displayName, &createdAt); err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}
		list = append(list, gin.H{
			"id": id, "username": username, "display_name": displayName, "created_at": createdAt,
		})
	}
	c.JSON(200, gin.H{"items": list})
}

// adminNewReq 新增/编辑请求。
type adminNewReq struct {
	Username    string `json:"username"`
	Password    string `json:"password"`
	DisplayName string `json:"display_name"`
}

// createAdmin POST /api/admin/admins —— 新增管理员（账号唯一 + 密码哈希入库）。
func (d Deps) createAdmin(c *gin.Context) {
	var req adminNewReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	if len(req.Username) < 2 || len(req.Password) < 6 {
		c.JSON(400, gin.H{"error": "账号≥2字符、密码≥6位"})
		return
	}
	hash, err := hashPassword(req.Password)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	display := req.DisplayName
	if display == "" {
		display = req.Username
	}
	id, err := d.Store.Admins().Create(c.Request.Context(), &domain.Admin{
		Username: req.Username, PasswordHash: hash, DisplayName: display,
	})
	if err != nil {
		c.JSON(400, gin.H{"error": "账号可能已存在：" + err.Error()})
		return
	}
	c.JSON(200, gin.H{"id": id})
}

// updateAdmin PUT /api/admin/admins/:id —— 改密码 / 改显示名（密码留空=不改）。
func (d Deps) updateAdmin(c *gin.Context) {
	id, err := parseID64(c)
	if err != nil {
		return
	}
	var req adminNewReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	sets, args := []string{}, []any{}
	if req.Password != "" {
		if len(req.Password) < 6 {
			c.JSON(400, gin.H{"error": "密码≥6位"})
			return
		}
		hash, err := hashPassword(req.Password)
		if err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}
		sets, args = append(sets, "password_hash=?"), append(args, hash)
	}
	if req.DisplayName != "" {
		sets, args = append(sets, "display_name=?"), append(args, req.DisplayName)
	}
	if len(sets) == 0 {
		c.JSON(400, gin.H{"error": "无可更新字段（密码/显示名至少填一项）"})
		return
	}
	args = append(args, id)
	if _, err := d.Store.DB.ExecContext(c.Request.Context(),
		`UPDATE admins SET `+joinSets(sets)+` WHERE id=?`, args...); err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	c.JSON(200, gin.H{"ok": true})
}

// deleteAdmin DELETE /api/admin/admins/:id —— 不能删除当前登录账号自己（actor_id 保护）。
func (d Deps) deleteAdmin(c *gin.Context) {
	id, err := parseID64(c)
	if err != nil {
		return
	}
	if v, ok := c.Get("actor_id"); ok {
		if selfID, ok := v.(int64); ok && selfID == id {
			c.JSON(400, gin.H{"error": "不能删除当前登录的账号"})
			return
		}
	}
	if _, err := d.Store.DB.ExecContext(c.Request.Context(),
		`DELETE FROM admins WHERE id=?`, id); err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	c.JSON(200, gin.H{"ok": true})
}

// parseID64 路径参数 id 解析（非法直接 400 返回）。
func parseID64(c *gin.Context) (int64, error) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		c.JSON(400, gin.H{"error": "id 非法"})
		return 0, err
	}
	return id, nil
}

// joinSets sets 片段逗号连接。
func joinSets(sets []string) string {
	out := ""
	for i, s := range sets {
		if i > 0 {
			out += ", "
		}
		out += s
	}
	return out
}
