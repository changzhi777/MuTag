// Package adminui 管理后台 SPA 托管（构建产物内嵌）。
// 构建流程：admin-web 目录 pnpm build → 产物拷贝至本包 dist/ → go build。
// 注：不经 http.FileServer（其对 /index.html 有 301 行为），直接读字节按 MIME 输出。
package adminui

import (
	"embed"
	"mime"
	"net/http"
	"path"
	"strings"

	"github.com/gin-gonic/gin"
)

//go:embed all:dist
var distFS embed.FS

// Register 挂载 /admin 路由：静态资源直出，未匹配路径回退 index.html（history 路由）。
func Register(r *gin.Engine) {
	handler := func(c *gin.Context) {
		p := strings.TrimPrefix(c.Request.URL.Path, "/admin")
		if p == "" || p == "/" {
			p = "/index.html"
		}
		data, err := distFS.ReadFile("dist" + path.Clean("/"+p))
		if err != nil {
			// SPA fallback：非静态文件路径回 index.html
			data, err = distFS.ReadFile("dist/index.html")
			if err != nil {
				c.String(http.StatusNotFound, "admin ui not built")
				return
			}
			p = "/index.html"
		}
		ct := mime.TypeByExtension(path.Ext(p))
		if ct == "" {
			ct = "application/octet-stream"
		}
		c.Data(http.StatusOK, ct, data)
	}

	r.GET("/admin", handler)
	r.GET("/admin/*path", handler)
}
