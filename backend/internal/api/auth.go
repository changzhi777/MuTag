package api

// 管理后台鉴权：单管理员密码 → HMAC 无状态 token。
// ADMIN_PASSWORD 未设置 = 本地开发免鉴权；设置后 /api/admin/*（除 login）必须带 Bearer。
// 上线前必须设置强密码；微信 users 体系（admin/asset_mgr/staff 角色）留 M2。

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
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

// issueToken 签发 exp.b64(hmac(exp))。
func issueToken(key []byte, now time.Time) string {
	exp := strconv.FormatInt(now.Add(tokenTTL).Unix(), 10)
	mac := hmac.New(sha256.New, key)
	mac.Write([]byte(exp))
	sig := hex.EncodeToString(mac.Sum(nil))
	return exp + "." + sig
}

// verifyToken 校验 token 有效且未过期。
func verifyToken(key []byte, token string) bool {
	exp, sig, ok := strings.Cut(token, ".")
	if !ok {
		return false
	}
	expUnix, err := strconv.ParseInt(exp, 10, 64)
	if err != nil || time.Now().Unix() > expUnix {
		return false
	}
	mac := hmac.New(sha256.New, key)
	mac.Write([]byte(exp))
	want := hex.EncodeToString(mac.Sum(nil))
	return hmac.Equal([]byte(sig), []byte(want))
}

var errNoAuth = errors.New("auth disabled or invalid credentials")

// adminAuth 鉴权中间件：未配置密码时放行（本地开发形态）。
func adminAuth(password string) gin.HandlerFunc {
	key := hmacKey(password)
	return func(c *gin.Context) {
		if key == nil {
			c.Next()
			return
		}
		token := strings.TrimPrefix(c.GetHeader("Authorization"), "Bearer ")
		if !verifyToken(key, token) {
			c.AbortWithStatusJSON(401, gin.H{"error": "unauthorized"})
			return
		}
		c.Next()
	}
}

// adminLogin POST /api/admin/login。
func adminLogin(password string) gin.HandlerFunc {
	key := hmacKey(password)
	return func(c *gin.Context) {
		if key == nil {
			// 免鉴权形态：直接放行空 token（前端跳过登录页）
			c.JSON(200, gin.H{"token": "", "open": true})
			return
		}
		var req struct {
			Password string `json:"password"`
		}
		if err := c.ShouldBindJSON(&req); err != nil || req.Password != password {
			c.JSON(401, gin.H{"error": "密码错误"})
			return
		}
		c.JSON(200, gin.H{"token": issueToken(key, time.Now())})
	}
}
