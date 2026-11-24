// 详情页：WebView 加载活码 URL（/t/{code} 302 → H5 详情，与浏览器同页同源）。
// 返回逻辑（三层，符合直觉）：
//   1. H5 页内点过链接（WebView 有历史）→ 顶部返回/物理键 = WebView 后退
//   2. WebView 无历史 → 顶部返回/物理键 = 导航返回（回扫码页）
//   3. 扫码页无栈可回 → 物理键退出 App
import React, { useEffect, useRef, useState } from "react";
import { BackHandler, StyleSheet, Pressable, Text, View } from "react-native";
import { WebView } from "react-native-webview";
import { useNavigation } from "@react-navigation/native";

export default function DetailScreen({ route }: any) {
  const { url } = route.params as { url: string };
  const nav = useNavigation();
  const webRef = useRef<any>(null);
  const [webCanGoBack, setWebCanGoBack] = useState(false);

  // 安卓物理返回键：WebView 有历史 → 网页后退；无历史 → false 交还导航（回扫码页）
  useEffect(() => {
    const sub = BackHandler.addEventListener("hardwareBackPress", () => {
      if (webCanGoBack) {
        webRef.current?.goBack();
        return true;
      }
      return false;
    });
    return () => sub.remove();
  }, [webCanGoBack]);

  const goBack = () => {
    if (webCanGoBack) {
      webRef.current?.goBack();
    } else {
      nav.goBack();
    }
  };

  return (
    <View style={S.root}>
      <View style={S.bar}>
        <Pressable style={S.back} hitSlop={12} onPress={goBack}>
          <Text style={S.backText}>‹ 返回扫码</Text>
        </Pressable>
        <Text style={S.title} numberOfLines={1}>
          {url}
        </Text>
      </View>
      <WebView
        ref={webRef}
        source={{ uri: url }}
        style={S.web}
        originWhitelist={["https://mutag.qingmulife.cn/*"]}
        onShouldStartLoadWithRequest={(req) => {
          // 活码域内放行；外链（防伪签名/广告等）交系统浏览器
          return req.url.startsWith("https://mutag.qingmulife.cn/");
        }}
        onNavigationStateChange={(s) => setWebCanGoBack(s.canGoBack)}
        startInLoadingState
      />
    </View>
  );
}

const S = StyleSheet.create({
  root: { flex: 1, backgroundColor: "#fff" },
  bar: {
    height: 44,
    flexDirection: "row",
    alignItems: "center",
    paddingHorizontal: 12,
    borderBottomWidth: StyleSheet.hairlineWidth,
    borderBottomColor: "#e5e7eb",
  },
  back: { paddingRight: 16, paddingVertical: 10 },
  backText: { color: "#0D9488", fontSize: 15 },
  title: { flex: 1, color: "#6b7280", fontSize: 12 },
  web: { flex: 1 },
});
