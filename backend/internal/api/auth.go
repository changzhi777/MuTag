package api

// 管理后台鉴权：单管理员密码 + 0007 多管理员（admins 表）→ HMAC 无状态 token（带用户名）。
// 双轨兼容：登录带 username → admins 表校验；不带 username → ADMIN_PASSWORD 单密码校验。
// ADMIN_PASSWORD 未设置且无 admins = 本地开发免鉴权；设置后 /api/admin/*（除 login）必须带 Bearer。
// token 格式：exp.username.sig（sig = hmac(exp + "|" + username)），24h 过期。

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"mutag/backend/internal/repo"
)

const tokenTTL = 24 * time.Hour

// hmacKey 由密码派生（空密码时返回 nil）。
func hmacKey(password string) []byte {
	if password == "" {
		return nil
	}
	sum := sha256.Sum256([]byte("mutag:" + password))
	return sum[:]
}

// issueToken 签发 exp.username.sig（sig = hmac(exp + "|" + username)）。
func issueToken(key []byte, username string, now time.Time) string {
	exp := strconv.FormatInt(now.Add(tokenTTL).Unix(), 10)
	mac := hmac.New(sha256.New, key)
	mac.Write([]byte(exp + "|" + username))
	sig := hex.EncodeToString(mac.Sum(nil))
	return exp + "." + username + "." + sig
}

// verifyToken 校验 token 有效且未过期，返回登录用户名。
func verifyToken(key []byte, token string) (string, bool) {
	exp, rest, ok := strings.Cut(token, ".")
	if !ok {
		return "", false
	}
	username, sig, ok := strings.Cut(rest, ".")
	if !ok {
		return "", false // 旧格式（无 username）token 已失效，重登即可
	}
	expUnix, err := strconv.ParseInt(exp, 10, 64)
	if err != nil || time.Now().Unix() > expUnix {
		return "", false
	}
	mac := hmac.New(sha256.New, key)
	mac.Write([]byte(exp + "|" + username))
	want := hex.EncodeToString(mac.Sum(nil))
	if !hmac.Equal([]byte(sig), []byte(want)) {
		return "", false
	}
	return username, true
}

var errNoAuth = errors.New("auth disabled or invalid credentials")

// adminAuth 鉴权中间件：未配置密码时放行（本地开发形态）；通过后 context 注入
// actor（管理员显示名，履历 Actor 用——单密码形态为 "admin"）与 actor_id（数字 id，删除保护用）。
func adminAuth(password string, store *repo.Store) gin.HandlerFunc {
	key := hmacKey(password)
	return func(c *gin.Context) {
		if key == nil {
			c.Next()
			return
		}
		token := strings.TrimPrefix(c.GetHeader("Authorization"), "Bearer ")
		username, ok := verifyToken(key, token)
		if !ok {
			c.AbortWithStatusJSON(401, gin.H{"error": "unauthorized"})
			return
		}
		actor, actorID := username, int64(0)
		if username != "" && store != nil {
			if admin, err := store.Admins().GetByUsername(c.Request.Context(), username); err == nil {
				if admin.DisplayName != "" {
					actor = admin.DisplayName
				}
				actorID = admin.ID
			}
		}
		if actor == "" {
			actor = "admin" // 单密码形态的匿名管理员
		}
		c.Set("actor", actor)
		c.Set("actor_id", actorID)
		c.Next()
	}
}

// adminLogin POST /api/admin/login —— 双轨：username 有值走 admins 表（0007 多管理员），
// username 为空走 ADMIN_PASSWORD 单密码（兼容形态）。
func adminLogin(password string, store *repo.Store) gin.HandlerFunc {
	key := hmacKey(password)
	return func(c *gin.Context) {
		var req struct {
			Username string `json:"username"`
			Password string `json:"password"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(401, gin.H{"error": "密码错误"})
			return
		}
		// 多管理员轨：admins 表校验（username + password_hash）
		if req.Username != "" {
			if store == nil {
				c.JSON(401, gin.H{"error": "账号或密码错误"})
				return
			}
			admin, err := store.Admins().GetByUsername(c.Request.Context(), req.Username)
			if err != nil || !checkPasswordHash(req.Password, admin.PasswordHash) {
				c.JSON(401, gin.H{"error": "账号或密码错误"})
				return
			}
			name := admin.Username
			if admin.DisplayName != "" {
				name = admin.DisplayName
			}
			// token 塞 username（稳定身份标识，删除保护用）；显示名走响应字段与 adminAuth 反查
			c.JSON(200, gin.H{"token": issueToken(key, admin.Username, time.Now()), "display_name": name})
			return
		}
		// 单密码轨（兼容形态）
		if key == nil {
			// 免鉴权形态：直接放行空 token（前端跳过登录页）
			c.JSON(200, gin.H{"token": "", "open": true})
			return
		}
		if req.Password != password {
			c.JSON(401, gin.H{"error": "密码错误"})
			return
		}
		c.JSON(200, gin.H{"token": issueToken(key, "", time.Now())})
	}
}

// actorOf 取当前登录管理员用户名（单密码形态 → "admin"；未登录兜底同值）。
func actorOf(c *gin.Context) string {
	if v, ok := c.Get("actor"); ok {
		if s, ok := v.(string); ok && s != "" {
			return s
		}
	}
	return "admin"
}

// hashPassword 生成 sha256$<salt-hex>$<hash-hex> 格式密码哈希（salt 16B 随机）。
func hashPassword(pw string) (string, error) {
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	h := sha256.New()
	h.Write(salt)
	h.Write([]byte(pw))
	return "sha256$" + hex.EncodeToString(salt) + "$" + hex.EncodeToString(h.Sum(nil)), nil
}

// checkPasswordHash 校验明文密码与存储哈希。
func checkPasswordHash(pw, stored string) bool {
	parts := strings.Split(stored, "$")
	if len(parts) != 3 || parts[0] != "sha256" {
		return false
	}
	salt, err := hex.DecodeString(parts[1])
	if err != nil {
		return false
	}
	h := sha256.New()
	h.Write(salt)
	h.Write([]byte(pw))
	return hmac.Equal([]byte(hex.EncodeToString(h.Sum(nil))), []byte(parts[2]))
}
