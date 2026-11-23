// 扫码页：vision-camera V5 + barcode-scanner 扩展（MLKit）。
// 用 useBarcodeScannerOutput + <Camera> 组合（不用现成 <CodeScanner> 组件——
// 其 device 为 null 时直接 throw，首帧未就绪会崩；自行判空保手动输入兜底可用）。
// 识别活码 URL（…/t/{code}）或裸编号（MT-…）→ 统一跳 WebView 详情页（分流器 302 → H5，扫码统计入 scan_logs）。
import React, { useState } from 'react';
import {
  View,
  Text,
  TextInput,
  Pressable,
  StyleSheet,
  Alert,
} from 'react-native';
import { Camera, useCameraDevice } from 'react-native-vision-camera';
import { useBarcodeScannerOutput } from 'react-native-vision-camera-barcode-scanner';
import { useFocusEffect } from '@react-navigation/native';
import { BASE_URL, TAG_ID_RE, normalizeTagInput } from '../constants';

// 活码内容形如 https://mutag.qingmulife.cn/t/MT-2026-000001
function extractCode(value: string): string | null {
  const m = value.match(/\/t\/(MT-\d{4}-\d{6})/i);
  if (m) return m[1].toUpperCase();
  const n = normalizeTagInput(value);
  if (TAG_ID_RE.test(n)) return n;
  return null;
}

export default function ScanScreen({ navigation }: any) {
  const device = useCameraDevice('back');
  const [manual, setManual] = useState('');
  const [cameraActive, setCameraActive] = useState(true);

  // 页面聚焦时激活相机，跳详情后返回重新激活（省电 + 免权限重复弹）
  useFocusEffect(
    React.useCallback(() => {
      setCameraActive(true);
      return () => setCameraActive(false);
    }, [])
  );

  const openCode = (code: string) => {
    setCameraActive(false);
    // 统一走活码分流器：WebView 内 302 → H5 详情，scan_logs 统计对齐
    navigation.navigate('Detail', { url: `${BASE_URL}/t/${code}` });
  };

  const scannerOutput = useBarcodeScannerOutput({
    barcodeFormats: ['qr-code'],
    onBarcodeScanned(barcodes) {
      if (!cameraActive) return;
      for (const b of barcodes) {
        const code = extractCode(b.rawValue ?? '');
        if (code) {
          // 防连扫：命中即休眠相机再跳页
          setCameraActive(false);
          openCode(code);
          break;
        }
      }
    },
    onError(error) {
      // 扫码管线错误仅记日志，不打断手动输入兜底通道
      console.warn('[scan] barcode scanner error:', error);
    },
  });

  const onManualSubmit = () => {
    const code = normalizeTagInput(manual);
    if (!TAG_ID_RE.test(code)) {
      Alert.alert('编号格式', '请输入 MT-YYYY-NNNNNN 格式的资产编号');
      return;
    }
    openCode(code);
  };

  // 权限被拒或设备未就绪时保留手动输入通道
  const cameraReady = device != null && cameraActive;

  return (
    <View style={S.root}>
      <View style={S.cameraBox}>
        {cameraReady ? (
          <Camera
            style={StyleSheet.absoluteFill}
            device={device}
            isActive
            outputs={[scannerOutput]}
          />
        ) : (
          <View style={[S.camFallback, S.camFallbackPos]}>
            <Text style={S.camFallbackText}>
              {device == null ? '未检测到相机设备' : '相机已暂停'}
            </Text>
            {device != null && (
              <Pressable style={S.resumeBtn} onPress={() => setCameraActive(true)}>
                <Text style={S.resumeText}>重新扫码</Text>
              </Pressable>
            )}
          </View>
        )}
        <View style={S.frameBox} pointerEvents="none">
          <View style={S.frame} />
        </View>
      </View>

      <View style={S.hintWrap}>
        <Text style={S.hint}>对准资产上的青沐码，自动识别</Text>
      </View>

      <View style={S.manualWrap}>
        <Text style={S.manualLabel}>或手动输入资产编号</Text>
        <TextInput
          style={S.input}
          value={manual}
          onChangeText={setManual}
          placeholder="MT-2026-000001"
          placeholderTextColor="#9ca3af"
          autoCapitalize="characters"
          autoCorrect={false}
          onSubmitEditing={onManualSubmit}
        />
        <Pressable style={S.openBtn} onPress={onManualSubmit}>
          <Text style={S.openBtnText}>查看资产</Text>
        </Pressable>
      </View>
    </View>
  );
}

const S = StyleSheet.create({
  root: { flex: 1, backgroundColor: '#0b1220' },
  cameraBox: { flex: 2, overflow: 'hidden', backgroundColor: '#111827' },
  camFallback: {
    flex: 1,
    alignItems: 'center',
    justifyContent: 'center',
    backgroundColor: '#111827',
  },
  camFallbackPos: { position: 'absolute', left: 0, right: 0, top: 0, bottom: 0 },
  camFallbackText: { color: '#9ca3af', marginBottom: 12 },
  resumeBtn: { paddingHorizontal: 16, paddingVertical: 8, borderRadius: 8, backgroundColor: '#0D9488' },
  resumeText: { color: '#fff' },
  frameBox: {
    position: 'absolute',
    left: 0,
    right: 0,
    top: 0,
    bottom: 0,
    alignItems: 'center',
    justifyContent: 'center',
  },
  frame: { width: 240, height: 240, borderWidth: 2, borderColor: 'rgba(13,148,136,0.9)', borderRadius: 16 },
  hintWrap: { paddingVertical: 10, alignItems: 'center' },
  hint: { color: '#9ca3af', fontSize: 13 },
  manualWrap: { flex: 1, paddingHorizontal: 20, paddingTop: 8, backgroundColor: '#0b1220' },
  manualLabel: { color: '#6b7280', fontSize: 12, marginBottom: 8 },
  input: {
    height: 48,
    borderRadius: 10,
    borderWidth: 1,
    borderColor: '#374151',
    paddingHorizontal: 14,
    color: '#f9fafb',
    fontSize: 16,
  },
  openBtn: {
    marginTop: 12,
    height: 48,
    borderRadius: 10,
    backgroundColor: '#0D9488',
    alignItems: 'center',
    justifyContent: 'center',
  },
  openBtnText: { color: '#fff', fontSize: 16, fontWeight: '600' },
});
