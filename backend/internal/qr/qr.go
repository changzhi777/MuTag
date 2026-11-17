// Package qr 活码 QR 图渲染：skip2/go-qrcode 薄封装。
// 职责收敛：尺寸限幅、纠错级别默认、PNG 字节输出——业务语义（内容构造/鉴权/限流）留在 api 层。
package qr

import (
	"fmt"

	qrcode "github.com/skip2/go-qrcode"
)

// 尺寸限幅：下限防 skip2 对过小尺寸报错，上限防恶意拉大图打内存。
const (
	minSize = 64
	maxSize = 1024
	defSize = 512
)

// ClampSize 规整用户传入尺寸：缺省 512，越界钳到 [64,1024]。
func ClampSize(size int) int {
	if size <= 0 {
		return defSize
	}
	if size < minSize {
		return minSize
	}
	if size > maxSize {
		return maxSize
	}
	return size
}

// Render 活码内容 → PNG 字节（Medium 纠错，自带白边——扫码软件友好）。
func Render(content string, size int) ([]byte, error) {
	png, err := qrcode.Encode(content, qrcode.Medium, ClampSize(size))
	if err != nil {
		return nil, fmt.Errorf("qr render: %w", err)
	}
	return png, nil
}
