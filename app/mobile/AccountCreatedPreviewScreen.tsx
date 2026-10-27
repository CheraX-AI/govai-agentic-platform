import { useState } from 'react';
import { StatusBar } from 'expo-status-bar';
import { Image, Pressable, ScrollView, StyleSheet, Text, View, useWindowDimensions } from 'react-native';
import { Screen } from './components/Screen';
import { layout, typography } from './layout';
import { loginAssets } from './LoginScreen';
import { registerAssets } from './RegisterScreen';

const checkCircle = require('./assets/figma/account-created/check-circle.png');

export function AccountCreatedPreviewScreen({ onExit }: { onExit: () => void }) {
  // Defence in depth: direct mounting cannot bypass the development entry guard.
  if (!__DEV__ || process.env.EXPO_PUBLIC_ACCOUNT_CREATED_PREVIEW !== '1') return null;
  const { width, height, fontScale } = useWindowDimensions();
  const [viewport, setViewport] = useState({ width: 0, height: 0 });
  const [visionHeight, setVisionHeight] = useState(47);
  const [cardHeight, setCardHeight] = useState(0);
  const [bannerHeight, setBannerHeight] = useState(0);
  const intrinsicBannerHeight = bannerHeight || 72 * fontScale;
  // Keep the label pinned when it fits. At very large text, put it in the
  // same scroll flow rather than letting it consume the entire usable window.
  const pinBanner = intrinsicBannerHeight <= (viewport.height || height * 0.7) / 2;
  const contentWidth = viewport.width || Math.min(layout.contentWidth, Math.max(0, width - 32));
  // This frame's own 150 x 151.531 logo, not Register's smaller header.
  const maximumLogoWidth = Math.min(150, Math.max(0, contentWidth - 2 * (86 + 4)));
  const logoWidth = Math.min(maximumLogoWidth, Math.max(Math.min(96, maximumLogoWidth),
    (viewport.height - intrinsicBannerHeight - 12 - (cardHeight || 346 * fontScale) - 32.469 - 40) * 150 / 151.531));
  const logoHeight = logoWidth * 151.531 / 150;
  const visionTop = Math.max(0, Math.min(26, (logoHeight - visionHeight) / 2));
  const headerHeight = Math.max(logoHeight, visionTop + visionHeight);
  const artworkScale = Math.min(width / 400, 1.25);
  const previewBanner = <View style={styles.previewBanner} onLayout={({ nativeEvent: { layout: measured } }) => {
    setBannerHeight(previous => Math.abs(previous - measured.height) < 0.5 ? previous : measured.height);
  }}>
    <Text style={styles.previewNotice}>Development preview — sample content. No account has been created.</Text>
    <Pressable onPress={onExit} accessibilityRole="button" accessibilityLabel="Exit preview and return to Welcome" style={styles.exit}>
      <Text style={styles.exitText}>Exit preview</Text>
    </Pressable>
  </View>;

  return (
    <View style={styles.root}>
      <StatusBar style="dark" />
      <View pointerEvents="none" accessible={false} importantForAccessibility="no-hide-descendants" style={StyleSheet.absoluteFill}>
        <Image source={loginAssets.topRibbon} accessible={false} resizeMode="cover"
          style={[styles.topRibbon, { width: 272 * artworkScale, height: 296 * artworkScale }]} />
        <Image source={loginAssets.skyline} accessible={false} resizeMode="contain"
          style={[styles.skyline, { width: 723 * artworkScale, height: 451 * artworkScale }]} />
        <Image source={loginAssets.bottomRibbon} accessible={false} resizeMode="cover"
          style={[styles.bottomRibbon, { width: 237 * artworkScale, height: 227 * artworkScale }]} />
      </View>
      <Screen scroll={false} horizontalPadding={16} contentStyle={styles.screen}>
        <View style={styles.viewport} onLayout={({ nativeEvent: { layout: measured } }) => {
          setViewport(previous => Math.abs(previous.width - measured.width) < 0.5 && Math.abs(previous.height - measured.height) < 0.5
            ? previous : { width: measured.width, height: measured.height });
        }}>
          {pinBanner && previewBanner}
          <ScrollView style={styles.viewport} showsVerticalScrollIndicator={false} alwaysBounceVertical={false} overScrollMode="never"
            contentContainerStyle={styles.content}>
            {!pinBanner && previewBanner}
            <View style={[styles.header, { height: headerHeight }]}>
              <View accessible accessibilityRole="image" accessibilityLabel="GovAI. People, services, a smarter Sri Lanka."
                style={[styles.logoSlot, { width: logoWidth, height: logoHeight }]}>
                <Image source={loginAssets.logo} accessible={false} resizeMode="contain" style={[styles.logo, {
                  width: logoWidth * 1.5417, height: logoHeight * 2.7132,
                  left: -logoWidth * 0.3013, top: -logoHeight * 0.636,
                }]} />
              </View>
              <View style={[styles.vision, { top: visionTop }]} onLayout={({ nativeEvent: { layout: measured } }) => {
                setVisionHeight(previous => Math.abs(previous - measured.height) < 0.5 ? previous : measured.height);
              }}>
                <Text style={styles.visionText}>{'FOR A\nSMARTER\nSRI LANKA'}</Text>
                <View style={styles.accent} />
              </View>
            </View>
            <View style={styles.card} onLayout={({ nativeEvent: { layout: measured } }) => {
              // Intrinsic card height has no dependency on header height.
              setCardHeight(previous => Math.abs(previous - measured.height) < 0.5 ? previous : measured.height);
            }}>
              <View style={styles.badge} accessible={false}>
                <Image source={checkCircle} accessible={false} style={styles.checkCircle} />
              </View>
              <View style={styles.cardHeader}>
                <Text accessibilityRole="header" style={styles.heading}>Account created</Text>
                <Text style={styles.description}>Welcome to GovAI, Rathishan. You can verify your identity later from Profile to unlock every service.</Text>
              </View>
              <Pressable disabled accessibilityRole="button" accessibilityState={{ disabled: true }}
                accessibilityLabel="Continue to GovAI. Unavailable in this preview; destination is not implemented." style={styles.primary}>
                <Text style={[typography.action, styles.primaryText]}>Continue to GovAI</Text>
                <Image source={registerAssets.next} accessible={false} style={styles.nextIcon} />
              </Pressable>
              <Text style={styles.unavailable}>Continue is unavailable: its destination is not implemented.</Text>
            </View>
            <Text style={styles.country}>SRI LANKA</Text>
          </ScrollView>
        </View>
      </Screen>
    </View>
  );
}

const styles = StyleSheet.create({
  root: { flex: 1, backgroundColor: '#fff' },
  screen: { flex: 1, gap: 12 },
  viewport: { flex: 1 },
  content: { flexGrow: 1, paddingBottom: 12 },
  topRibbon: { position: 'absolute', left: -9, top: -14 },
  skyline: { position: 'absolute', left: -164, bottom: -6, opacity: 0.45 },
  bottomRibbon: { position: 'absolute', right: -7, bottom: -8 },
  previewBanner: { flexDirection: 'row', alignItems: 'center', gap: 8, padding: 8, marginBottom: 12, borderRadius: 12, borderWidth: 1, borderColor: '#f5a915', backgroundColor: '#fff8e8', flexShrink: 0 },
  previewNotice: { flex: 1, minWidth: 0, fontFamily: 'Inter_500Medium', fontSize: 13, lineHeight: 18, color: '#142033' },
  exit: { minWidth: layout.touchTarget, minHeight: layout.touchTarget, paddingHorizontal: 8, justifyContent: 'center', maxWidth: '40%' },
  exitText: { fontFamily: 'Inter_600SemiBold', fontSize: 13, lineHeight: 18, color: '#8b1a2e', textAlign: 'center' },
  header: { width: '100%', alignItems: 'center', flexShrink: 0, marginBottom: 32.469 },
  logoSlot: { overflow: 'hidden' },
  logo: { position: 'absolute' },
  vision: { position: 'absolute', right: 0, width: 86, gap: 6, alignItems: 'flex-end' },
  visionText: { fontFamily: 'Inter_500Medium', fontSize: 9, lineHeight: 13, letterSpacing: 2, color: '#666', textAlign: 'right' },
  accent: { width: 36, height: 2, backgroundColor: '#f5a915' },
  card: { width: '100%', paddingHorizontal: 20, paddingVertical: 24, gap: 16, alignItems: 'center', backgroundColor: '#fff', borderWidth: 1, borderColor: '#f1f5f9', borderRadius: 24, boxShadow: '0px 10px 22px rgba(0,0,0,0.1)', flexShrink: 0 },
  badge: { width: 72, height: 72, borderRadius: 9999, backgroundColor: '#f2f8f4', alignItems: 'center', justifyContent: 'center' },
  checkCircle: { width: 34, height: 34 },
  cardHeader: { width: '100%', gap: 6, alignItems: 'center' },
  heading: { fontFamily: 'Inter_700Bold', fontSize: 20, lineHeight: 28, color: '#142033', textAlign: 'center' },
  description: { width: '100%', fontFamily: 'Inter_400Regular', fontSize: 13, lineHeight: 18, color: '#666', textAlign: 'center' },
  primary: { width: '100%', minHeight: 52, paddingHorizontal: 24, paddingVertical: 16, flexDirection: 'row', alignItems: 'center', justifyContent: 'center', gap: 10, borderRadius: 14, backgroundColor: '#8b1a2e', boxShadow: '0px 6px 16px rgba(139,26,46,0.25)' },
  primaryText: { flexShrink: 1, fontFamily: 'Inter_600SemiBold', color: '#fff', textAlign: 'center' },
  nextIcon: { width: 16, height: 16 },
  unavailable: { width: '100%', fontFamily: 'Inter_400Regular', fontSize: 13, lineHeight: 18, color: '#666', textAlign: 'center' },
  country: { marginTop: 'auto', paddingTop: 24, fontFamily: 'Inter_400Regular', fontSize: 9, lineHeight: 13, letterSpacing: 4, color: '#666', textAlign: 'center' },
});
