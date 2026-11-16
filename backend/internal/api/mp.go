// 小程序员工端 API：登录、身份、快捷操作（写 event 纪律）、我的上报。
// 登录 dev 模式：WECHAT_APPID 未配置时 openid = "dev_" + sha256(code) 截断，全流程可测；
// 上线配置 AppID/Secret 后自动切真 code2session。
package api

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"mutag/backend/internal/domain"
)

// wechatSession 微信 code2session 响应（生产形态）。
type wechatSession struct {
	OpenID  string `json:"openid"`
	ErrCode int    `json:"errcode"`
	ErrMsg  string `json:"errmsg"`
}

// exchangeOpenID code 换 openid：AppID 空走 dev mock，否则调微信。
func exchangeOpenID(ctx context.Context, appID, secret, code string) (string, bool) {
	if appID == "" {
		sum := sha256.Sum256([]byte("dev-login:" + code))
		return "dev_" + hex.EncodeToString(sum[:6]), true
	}
	url := fmt.Sprintf(
		"https://api.weixin.qq.com/sns/jscode2session?appid=%s&secret=%s&js_code=%s&grant_type=authorization_code",
		appID, secret, code)
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return "", false
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", false
	}
	var session wechatSession
	if err := json.Unmarshal(body, &session); err != nil || session.OpenID == "" {
		return "", false
	}
	return session.OpenID, true
}

// mpLogin POST /api/v1/auth/login {code}
func (d Deps) mpLogin(c *gin.Context) {
	var req struct {
		Code string `json:"code"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || strings.TrimSpace(req.Code) == "" {
		c.JSON(400, gin.H{"error": "code 必填"})
		return
	}
	openid, ok := exchangeOpenID(c.Request.Context(), d.WechatAppID, d.WechatSecret, req.Code)
	if !ok {
		c.JSON(401, gin.H{"error": "登录失败（code 无效）"})
		return
	}
	user, err := d.Store.Users().GetOrCreate(c.Request.Context(), openid, "")
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	c.JSON(200, gin.H{
		"token": issueUserToken(d.TokenSecret, openid, time.Now()),
		"user":  user,
	})
}

// mpMe GET /api/v1/auth/me
func (d Deps) mpMe(c *gin.Context) {
	openid := c.GetString("openid")
	user, err := d.Store.Users().GetOrCreate(c.Request.Context(), openid, "")
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	c.JSON(200, gin.H{"user": user})
}

// mpAction POST /api/v1/assets/:tagID/actions —— 快捷操作（staff 可发起，直接生效）。
// action: receive(领用 keeper=我) / return(归还 keeper 清空) / repair(报修 status=repair) / transfer(位置变更 location=新值)
func (d Deps) mpAction(c *gin.Context) {
	tagID := c.Param("tagID")
	openid := c.GetString("openid")
	var req struct {
		Action   string `json:"action"`
		Location string `json:"location"`
		Note     string `json:"note"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	ctx := c.Request.Context()

	asset, err := d.Store.Assets().GetByTagID(ctx, tagID)
	if err != nil {
		c.JSON(404, gin.H{"error": "asset not found"})
		return
	}

	var evType domain.EventType
	var oldMap, newMap map[string]any
	switch strings.TrimSpace(req.Action) {
	case "receive":
		evType = domain.EventReceive
		oldMap = map[string]any{"keeper": deref(asset.Keeper)}
		newMap = map[string]any{"keeper": openid}
		asset.Keeper = &openid
	case "return":
		evType = domain.EventReturn
		oldMap = map[string]any{"keeper": deref(asset.Keeper)}
		newMap = map[string]any{"keeper": ""}
		asset.Keeper = nil
	case "repair":
		evType = domain.EventRepair
		oldMap = map[string]any{"status": string(asset.Status)}
		newMap = map[string]any{"status": string(domain.AssetRepair)}
		asset.Status = domain.AssetRepair
	case "transfer":
		if strings.TrimSpace(req.Location) == "" {
			c.JSON(400, gin.H{"error": "location 必填"})
			return
		}
		evType = domain.EventTransfer
		oldMap = map[string]any{"location": deref(asset.Location)}
		newMap = map[string]any{"location": req.Location}
		asset.Location = &req.Location
	default:
		c.JSON(400, gin.H{"error": "action 必须为 receive/return/repair/transfer"})
		return
	}

	if err := d.Store.Assets().Update(ctx, asset); err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	note := req.Note
	_ = d.Store.Events().Create(ctx, &domain.AssetEvent{
		AssetID: asset.AssetID, Type: evType, Actor: openid,
		OldValue: oldMap, NewValue: newMap,
		Note: &note,
	})
	c.JSON(200, gin.H{"ok": true, "type": string(evType)})
}

// mpMyEvents GET /api/v1/auth/my-events —— 我的上报（新→旧 50 条）。
func (d Deps) mpMyEvents(c *gin.Context) {
	events, err := d.Store.Users().ListEventsByActor(c.Request.Context(), c.GetString("openid"), 50)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	if events == nil {
		events = make([]domain.AssetEvent, 0)
	}
	c.JSON(200, gin.H{"events": events})
}
