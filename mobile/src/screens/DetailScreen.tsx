// 详情页：WebView 加载活码 URL（/t/{code} 302 → H5 详情，与浏览器同页同源）。
// 返回键回扫码页继续扫；Linking 外链交给系统浏览器（避免 WebView 内嵌套跳转）。
import React from 'react';
import { View, StyleSheet, Pressable, Text, BackHandler } from 'react-native';
import { WebView } from 'react-native-webview';
import { useNavigation } from '@react-navigation/native';

export default function DetailScreen({ route }: any) {
  const { url } = route.params as { url: string };
  const nav = useNavigation();

  return (
    <View style={S.root}>
      <View style={S.bar}>
        <Pressable
          style={S.back}
          onPress={() => {
            if (nav.canGoBack()) nav.goBack();
            else BackHandler.exitApp();
          }}
        >
          <Text style={S.backText}>‹ 返回扫码</Text>
        </Pressable>
        <Text style={S.title} numberOfLines={1}>
          {url}
        </Text>
      </View>
      <WebView
        source={{ uri: url }}
        style={S.web}
        originWhitelist={['https://mutag.qingmulife.cn/*']}
        onShouldStartLoadWithRequest={(req) => {
          // 活码域内放行；外链（防伪签名/广告等）交系统浏览器
          return req.url.startsWith('https://mutag.qingmulife.cn/');
        }}
        startInLoadingState
      />
    </View>
  );
}

const S = StyleSheet.create({
  root: { flex: 1, backgroundColor: '#fff' },
  bar: {
    height: 44,
    flexDirection: 'row',
    alignItems: 'center',
    paddingHorizontal: 12,
    borderBottomWidth: StyleSheet.hairlineWidth,
    borderBottomColor: '#e5e7eb',
  },
  back: { paddingRight: 12, paddingVertical: 8 },
  backText: { color: '#0D9488', fontSize: 15 },
  title: { flex: 1, color: '#6b7280', fontSize: 12 },
  web: { flex: 1 },
});
