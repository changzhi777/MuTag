// 小程序端会话认证：openid 绑定的 HMAC 无状态 token。
// token 三段式：exp.openid.sig，sig = HMAC(secret, exp|openid)；middleware 校验后注入 openid。
package api

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

const userTokenTTL = 7 * 24 * time.Hour

// issueUserToken 签发小程序会话 token。
func issueUserToken(secret, openid string, now time.Time) string {
	exp := strconv.FormatInt(now.Add(userTokenTTL).Unix(), 10)
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(exp + "|" + openid))
	return exp + "." + openid + "." + hex.EncodeToString(mac.Sum(nil))
}

// parseUserToken 校验并解析 openid。
func parseUserToken(secret, token string) (string, bool) {
	expPart, rest, ok := strings.Cut(token, ".")
	if !ok {
		return "", false
	}
	openid, sig, ok := strings.Cut(rest, ".")
	if !ok || openid == "" {
		return "", false
	}
	expUnix, err := strconv.ParseInt(expPart, 10, 64)
	if err != nil || time.Now().Unix() > expUnix {
		return "", false
	}
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(expPart + "|" + openid))
	want := hex.EncodeToString(mac.Sum(nil))
	if !hmac.Equal([]byte(sig), []byte(want)) {
		return "", false
	}
	return openid, true
}

// userAuth 小程序会话中间件：校验后注入 openid。
func userAuth(secret string) gin.HandlerFunc {
	return func(c *gin.Context) {
		token := strings.TrimPrefix(c.GetHeader("Authorization"), "Bearer ")
		openid, ok := parseUserToken(secret, token)
		if !ok {
			c.AbortWithStatusJSON(401, gin.H{"error": "请先登录"})
			return
		}
		c.Set("openid", openid)
		c.Next()
	}
}
