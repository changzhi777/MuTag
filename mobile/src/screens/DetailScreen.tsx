// 详情页：WebView 加载活码 URL（/t/{code} 302 → H5 详情，与浏览器同页同源）。
// 返回逻辑（盘点连续扫码场景）：
//   1. 自动返回：进入详情 5 秒倒计时后自动回扫码页（继续扫下一件）；点「停留」或 H5 页内导航取消自动
//   2. 手动返回：顶部「‹ 返回扫码」/ 物理键——WebView 有历史先网页后退，无历史回扫码页
import React, { useEffect, useRef, useState } from "react";
import { BackHandler, StyleSheet, Pressable, Text, View } from "react-native";
import { WebView } from "react-native-webview";
import { useNavigation } from "@react-navigation/native";

const AUTO_BACK_SECONDS = 3;

export default function DetailScreen({ route }: any) {
  const { url } = route.params as { url: string };
  const nav = useNavigation();
  const webRef = useRef<any>(null);
  const [webCanGoBack, setWebCanGoBack] = useState(false);
  const [autoLeft, setAutoLeft] = useState<number | null>(AUTO_BACK_SECONDS); // null=已取消

  // 自动返回倒计时（扫一件看一眼继续扫下一件；点「停留」或页内导航取消）
  useEffect(() => {
    if (autoLeft == null) return;
    if (autoLeft <= 0) {
      nav.goBack();
      return;
    }
    const t = setTimeout(() => setAutoLeft((c) => (c == null ? null : c - 1)), 1000);
    return () => clearTimeout(t);
  }, [autoLeft, nav]);

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

  const manualBack = () => {
    if (webCanGoBack) {
      webRef.current?.goBack();
    } else {
      nav.goBack();
    }
  };

  return (
    <View style={S.root}>
      <View style={S.bar}>
        <Pressable style={S.back} hitSlop={12} onPress={manualBack}>
          <Text style={S.backText}>‹ 返回扫码</Text>
        </Pressable>
        {autoLeft != null ? (
          <Pressable style={S.autoBox} hitSlop={8} onPress={() => setAutoLeft(null)}>
            <Text style={S.autoText}>自动返回 {autoLeft}s · 停留</Text>
          </Pressable>
        ) : (
          <Text style={S.stayText}>浏览中</Text>
        )}
        <View style={S.spacer} />
      </View>
      <WebView
        ref={webRef}
        source={{ uri: url }}
        style={S.web}
        originWhitelist={["https://mutag.qingmulife.cn/*"]}
        onShouldStartLoadWithRequest={(req) => {
          // 用户主动点击（click）= 取消自动返回（细看场景）；302 重定向（other）不取消——
          // 活码 /t/{code} 自身 302 到 /h5/ 属程序行为，误取消会让自动返回从未开始
          if (req.navigationType === "click") setAutoLeft(null);
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
    justifyContent: "center",
    borderBottomWidth: StyleSheet.hairlineWidth,
    borderBottomColor: "#e5e7eb",
  },
  back: { position: "absolute", left: 12, paddingRight: 16, paddingVertical: 10 },
  backText: { color: "#0D9488", fontSize: 15 },
  autoBox: {
    paddingHorizontal: 10,
    paddingVertical: 5,
    borderRadius: 14,
    backgroundColor: "#ecfdf5",
    borderWidth: 1,
    borderColor: "#0D9488",
  },
  autoText: { color: "#0D9488", fontSize: 13, fontWeight: "600" },
  stayText: { color: "#9ca3af", fontSize: 13 },
  spacer: { width: 90 },
  web: { flex: 1 },
});
