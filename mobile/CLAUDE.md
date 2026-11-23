# mobile/ · 青沐码 Android APK（RN）— 模块文档

> 上级：[根 CLAUDE.md](../CLAUDE.md) ｜ 首建：V1.18（2026-11-23）｜ 实态：**APK 已出包（128MB，V2 debug 签名，真机待验 📋）**

---

## 定位

安卓端**原生扫码 + WebView 壳**客户端（方案 B）：vision-camera V5（MLKit）原生识别青沐活码 → 提取编号 → WebView 加载 `https://mutag.qingmulife.cn/t/{code}`（活码分流器 302 → H5 详情页）。**详情页与浏览器完全同源同页**——后端零改动，扫码统计入 `scan_logs` 与其他渠道对齐（channel 字段暂混用，二期可扩 app 渠道标识）。

与 miniprogram（微信端）互补：不依赖微信、不受域名拦截影响，适合仓库/车间等系统相机场景。

## 技术栈与关键裁决

| 项 | 值 | 裁决说明 |
|---|---|---|
| 框架 | Expo ~58.0.26 + RN 0.88.1 + react 19.3 | Expo prebuild 生成 android/ 原生工程，本地 gradlew 出 APK（不走 EAS 云构建） |
| 扫码 | react-native-vision-camera 5.2.4 + barcode-scanner 5.2.4 + nitro-modules 0.37.2 | **V5 是 Nitro 大重构**：无 `useCodeScanner` hook；正确用法 = `useBarcodeScannerOutput({barcodeFormats:['qr-code'], onBarcodeScanned, onError})` + `<Camera outputs={[output]}>` 组合；Barcode 字段 `rawValue` |
| 壳 | react-native-webview ^14.0.1 | originWhitelist 锁 mutag 域内，外链交系统浏览器 |
| 导航 | @react-navigation/native 7.x + native-stack + screens + safe-area-context | Scan / Detail 两屏 |
| 包名 | `cn.qingmulife.mutag`（versionName 1.0.0 / versionCode 1） | minSdk 24 / targetSdk 36 / compileSdk 37 |
| 签名 | V2 · debug keystore（RN 模板默认 release 用 debug 签名） | 内测/内部分发可用；**上商店前须换正式 keystore** |
| 权限 | CAMERA（android.permissions 声明，**非 config plugin**——vision-camera V5 已无 app.plugin.js，plugins 里注册会 prebuild 报错） | INTERNET 等 expo 默认集 |

## 源码结构

```
mobile/
├── App.tsx                  # NavigationContainer + native-stack（Scan/Detail，headerShown false）
├── index.ts                 # registerRootComponent
├── src/constants.ts         # BASE_URL / TAG_ID_RE(MT-\d{4}-\d{6}) / normalizeTagInput（trim+全角横线+大写）
├── src/screens/ScanScreen.tsx   # 相机预览 + 扫码（防连扫：命中即休眠相机再跳页）+ 手动输入兜底（权限被拒仍可用）
├── src/screens/DetailScreen.tsx # WebView + 顶部返回栏（canGoBack→goBack 否则 exitApp）
├── assets/                  # icon / adaptive-icon / splash（青沐 logo 256px）
├── android/                 # prebuild 产物（gitignore，可随时重建）
└── 青沐码-1.0.0.apk          # 出包产物（gitignore，构建后手动拷出）
```

**扫码识别规则**（ScanScreen.extractCode）：活码 URL `/t/(MT-\d{4}-\d{6})` 提取编号，或裸编号正则直过——打印工场出的码与服务端 `/qr` 出的码均覆盖。

## 构建链（实弹验证 2026-11-23）

```bash
cd mobile
npm install                      # 或 pnpm（注意 peer 冲突需 --legacy-peer-deps 场景）
npx expo prebuild --platform android --no-install
echo "sdk.dir=$HOME/Library/Android/sdk" > android/local.properties
cd android && ./gradlew assembleRelease
# 产物：app/build/outputs/apk/release/app-release.apk
```

**首次构建自动装**：NDK 27.1.12297006（license 已接受即自动）+ Build-Tools 37 + CMake。

### 构建环境坑（本机实测）

1. **沙箱内 JVM 无法构建**：Claude 沙箱的 gradle/NDK/maven 下载（3GB+）被过滤代理限速 40KB/s 且 JVM 不认环境变量代理——**构建一律在用户终端跑**
2. **JVM 直连国外被 Clash TUN 掐 TLS 握手**（SSLHandshakeException）→ `JAVA_TOOL_OPTIONS` 显式挂 Clash 代理：
   ```bash
   JAVA_TOOL_OPTIONS="-Dhttps.proxyHost=127.0.0.1 -Dhttps.proxyPort=7897 -Dhttp.proxyHost=127.0.0.1 -Dhttp.proxyPort=7897" ./gradlew assembleRelease
   ```
   （7897 = Clash Verge mixed-port，读 `~/Library/Application Support/io.github.clash-verge-rev.clash-verge-rev/` 配置）
3. **gradle wrapper 下载超时**（默认 read timeout 10s）→ 双修：`gradle-wrapper.properties` 的 distributionUrl 切腾讯云镜像 `https://mirrors.cloud.tencent.com/gradle/`（国内直连 16MB/s）+ `android/gradle.properties` 追加 `systemProp.org.gradle.internal.http.socketTimeout=120000`（NDK/maven 大文件防断）
4. **pnpm 严格 peer 解析**会找不到 `react-native-nitro-image@^0.29.0`（registry 无此版本线，实际最新 0.15.2 且 vision-camera peer 声明为 `*`）→ 用 npm 安装无此问题
5. **expo prebuild 沙箱可跑**（无需网络下载，node_modules 就位即可）；`android/` 在 gitignore，删除后 prebuild 可全量重建

## 待办 / 余项

- **真机验收** 📋：装 APK → 扫真实活码（MT-2026-000001）→ WebView 详情页 + 扫码计数入 scan_logs → 手动输入通道 → 返回再扫
- **包体优化**：128MB（4 ABI 全量）→ gradle `abiFilters/splits` 只留 arm64-v8a 可减至 ~40MB
- **正式签名**：内部测试 debug keystore 够用；对外分发前生成正式 keystore 并配 signingConfig
- **渠道标识**（二期）：scan_logs.channel 扩 `app` 值，区分 APK 扫码流量
- **logo 闪屏统一**：splash 目前纯 logo 白底，可对齐 admin-web 品牌样式
