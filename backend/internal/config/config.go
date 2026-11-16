// Package config 环境变量配置加载（stdlib 实现，零额外依赖）。
package config

import (
	"os"
	"strconv"
)

// Config 服务运行配置。
type Config struct {
	ListenAddr    string  // HTTP 监听地址
	SQLitePath    string  // SQLite 库文件路径（M1 开发形态）
	BaseURL       string  // 活码对外域名（如 https://mu.qingmu.cn）
	AdminPassword string  // 管理后台密码（空 = 本地开发免鉴权）
	WechatAppID   string  // 小程序 AppID（空 = dev 模式，openid 为 mock）
	WechatSecret  string  // 小程序 AppSecret
	TokenSecret   string  // 小程序会话 token 签名密钥
	WeappJumpURL  string  // 活码 302 微信内跳转目标（M2 配置后启用小程序唤起）
	RateBurst     float64 // 扫码限流桶容量（突发上限；<=0 关闭限流，纪律 4）
	RatePerSec    float64 // 扫码限流每秒回填令牌数
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func envFloat(key string, fallback float64) float64 {
	if v := os.Getenv(key); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			return f
		}
	}
	return fallback
}

// Load 从环境变量读取配置，缺省值面向本地开发。
func Load() Config {
	return Config{
		ListenAddr:    envOr("LISTEN_ADDR", ":8080"),
		SQLitePath:    envOr("SQLITE_PATH", "data/mutag.db"),
		BaseURL:       envOr("BASE_URL", "http://localhost:8080"),
		AdminPassword: os.Getenv("ADMIN_PASSWORD"),
		WechatAppID:   os.Getenv("WECHAT_APPID"),
		WechatSecret:  os.Getenv("WECHAT_SECRET"),
		TokenSecret:   envOr("TOKEN_SECRET", "dev-token-secret-change-me"),
		WeappJumpURL:  os.Getenv("WEAPP_JUMP_URL"),
		RateBurst:     envFloat("RATE_LIMIT_BURST", 30),
		RatePerSec:    envFloat("RATE_LIMIT_PER_SEC", 5),
	}
}
