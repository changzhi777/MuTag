package api

import (
	"testing"
)

func TestResolveTarget(t *testing.T) {
	cases := []struct {
		name    string
		ua      string
		h5Path  string
		miniURL string
		want    string
	}{
		{
			name:    "微信内+未配小程序跳转→H5（M1 现状）",
			ua:      "Mozilla/5.0 MicroMessenger/8.0",
			h5Path:  "/h5/t/MT-2026-000001",
			miniURL: "",
			want:    "/h5/t/MT-2026-000001",
		},
		{
			name:    "微信内+已配小程序跳转→小程序（M2 形态）",
			ua:      "Mozilla/5.0 MicroMessenger/8.0",
			h5Path:  "/h5/t/MT-2026-000001",
			miniURL: "weixin://dl/business/?t=xxx",
			want:    "weixin://dl/business/?t=xxx",
		},
		{
			name:    "外部浏览器→H5",
			ua:      "Mozilla/5.0 (iPhone) Safari",
			h5Path:  "/h5/t/MT-2026-000001",
			miniURL: "weixin://dl/business/?t=xxx",
			want:    "/h5/t/MT-2026-000001",
		},
		{
			name:    "扫码枪无 UA→H5",
			ua:      "",
			h5Path:  "/h5/t/MT-2026-000001",
			miniURL: "weixin://dl/business/?t=xxx",
			want:    "/h5/t/MT-2026-000001",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := resolveTarget(tc.ua, tc.h5Path, tc.miniURL); got != tc.want {
				t.Errorf("resolveTarget(%q) = %q, want %q", tc.ua, got, tc.want)
			}
		})
	}
}

func TestScanChannel(t *testing.T) {
	if got := scanChannel("Mozilla/5.0 MicroMessenger/8.0"); got != "weapp" {
		t.Errorf("weapp UA → %q, want weapp", got)
	}
	if got := scanChannel("Mozilla/5.0 (Linux; Android 14) AppleWebKit/537.36 QingMuApp/1.0"); got != "app" {
		t.Errorf("app UA → %q, want app", got)
	}
	// 微信优先：APK 内 WebView 载入微信内页场景仍归 weapp
	if got := scanChannel("Mozilla/5.0 MicroMessenger/8.0 QingMuApp/1.0"); got != "weapp" {
		t.Errorf("weapp+app UA → %q, want weapp", got)
	}
	if got := scanChannel("Mozilla/5.0 Safari"); got != "h5" {
		t.Errorf("browser UA → %q, want h5", got)
	}
	if got := scanChannel(""); got != "h5" {
		t.Errorf("empty UA → %q, want h5", got)
	}
}
