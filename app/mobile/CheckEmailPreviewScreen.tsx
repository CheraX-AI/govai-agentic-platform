import { useState } from 'react';
import { StatusBar } from 'expo-status-bar';
import { Image, Pressable, ScrollView, StyleSheet, Text, View, useWindowDimensions } from 'react-native';
import { Screen } from './components/Screen';
import { layout, typography } from './layout';
import { loginAssets } from './LoginScreen';
import { registerAssets } from './RegisterScreen';

const mail = require('./assets/figma/check-email/mail.png');

export function CheckEmailPreviewScreen({ onBack, onDifferentNic }: {
  onBack: () => void; onDifferentNic: () => void; onExit: () => void;
}) {
  // Direct mounting cannot bypass the development-only entry guard.
  if (!__DEV__ || process.env.EXPO_PUBLIC_CHECK_EMAIL_PREVIEW !== '1') return null;
  const { width, fontScale } = useWindowDimensions();
  const [viewport, setViewport] = useState({ width: 0, height: 0 });
  const [visionHeight, setVisionHeight] = useState(47);
  const [cardHeight, setCardHeight] = useState(0);
  const [bannerHeight, setBannerHeight] = useState(0);
  const contentWidth = viewport.width || Math.min(layout.contentWidth, Math.max(0, width - 32));
  const noticeHeight = bannerHeight || 18 * fontScale;
  const maximumLogoWidth = Math.min(150, width * 0.375, Math.max(0, contentWidth - 2 * (86 + 9 + 4)));
  const maximumLogoHeight = maximumLogoWidth * 151.531 / 150;
  const availableHeight = viewport.height - noticeHeight - 12;
  const intrinsicCardHeight = cardHeight || 383 * fontScale;
  const headerGap = Math.max(12, Math.min(32.469, availableHeight - intrinsicCardHeight - maximumLogoHeight - 42));
  const logoHeight = Math.min(maximumLogoHeight, Math.max(Math.min(64, maximumLogoHeight), availableHeight - intrinsicCardHeight - headerGap - 42));
  const logoWidth = logoHeight * 150 / 151.531;
  const visionTop = Math.max(6, Math.min(32, 6 + (logoHeight - visionHeight) / 2));
  const headerHeight = Math.max(logoHeight + 6, visionTop + visionHeight, layout.touchTarget + 5);
  const artworkScale = Math.min(width / 400, 1.25);
  return <View style={styles.root}>
    <StatusBar style="dark" />
    <View pointerEvents="none" accessible={false} importantForAccessibility="no-hide-descendants" style={StyleSheet.absoluteFill}>
      <Image source={loginAssets.topRibbon} accessible={false} resizeMode="cover" style={[styles.topRibbon, { width: 272 * artworkScale, height: 296 * artworkScale, left: -9 * artworkScale, top: -14 * artworkScale }]} />
      <Image source={loginAssets.skyline} accessible={false} resizeMode="contain" style={[styles.skyline, { width: 723 * artworkScale, height: 451 * artworkScale, left: -164 * artworkScale, bottom: -6 * artworkScale }]} />
      <Image source={loginAssets.bottomRibbon} accessible={false} resizeMode="cover" style={[styles.bottomRibbon, { width: 237 * artworkScale, height: 227 * artworkScale, right: -7 * artworkScale, bottom: -8 * artworkScale }]} />
    </View>
    <Screen scroll={false} horizontalPadding={16} contentStyle={styles.screen}>
      <View style={styles.viewport} onLayout={({ nativeEvent: { layout: measured } }) => {
        setViewport(previous => Math.abs(previous.width - measured.width) < 0.5 && Math.abs(previous.height - measured.height) < 0.5
          ? previous : { width: measured.width, height: measured.height });
      }}>
        <ScrollView style={styles.viewport} showsVerticalScrollIndicator={false} alwaysBounceVertical={false} overScrollMode="never" contentContainerStyle={styles.content}>
          <View style={[styles.header, { height: headerHeight }]}>
            <Pressable onPress={onBack} accessibilityRole="button" accessibilityLabel="Back to Forgot Password" style={styles.back}>
              <View style={styles.backCircle}><Image source={registerAssets.back} accessible={false} style={styles.backIcon} /></View>
            </Pressable>
            <View accessible accessibilityRole="image" accessibilityLabel="GovAI. People, services, a smarter Sri Lanka."
              style={[styles.logoSlot, { width: logoWidth, height: logoHeight }]}>
              <Image source={loginAssets.logo} accessible={false} resizeMode="contain" style={{ position: 'absolute', width: logoWidth * 1.5417,
                height: logoHeight * 2.7132, left: -logoWidth * 0.3013, top: -logoHeight * 0.636 }} />
            </View>
            <View style={[styles.vision, { top: visionTop }]} onLayout={event => {
              const measured = event.nativeEvent.layout.height; setVisionHeight(previous => Math.abs(previous - measured) < 0.5 ? previous : measured);
            }}><Text style={styles.visionText}>{'FOR A\nSMARTER\nSRI LANKA'}</Text><View style={styles.accent} /></View>
          </View>
          <Text style={[styles.previewNotice, { marginTop: headerGap }]} onLayout={event => {
            const measured = event.nativeEvent.layout.height;
            setBannerHeight(previous => Math.abs(previous - measured) < 0.5 ? previous : measured);
          }}>Preview · No email sent</Text>
          <View style={styles.card} onLayout={event => {
            // Intrinsic card height is independent of the flexible decorative header.
            const measured = event.nativeEvent.layout.height; setCardHeight(previous => Math.abs(previous - measured) < 0.5 ? previous : measured);
          }}>
            <View style={styles.badge}><Image source={mail} accessible={false} style={styles.mailIcon} /></View>
            <View style={styles.cardHeader}>
              <Text accessibilityRole="header" style={styles.heading}>Check your email</Text>
              <Text style={styles.description}>Example reset-link destination:</Text>
              <Text style={styles.sampleEmail}>sample@example.test</Text>
              <Text style={styles.caption}>No reset link exists or expires in this preview.</Text>
            </View>
            <View style={styles.actionGroup}>
              <Pressable disabled accessibilityRole="button" accessibilityState={{ disabled: true }}
                accessibilityLabel="Open email app. Unavailable in this preview; no mailbox will be opened." style={styles.disabledPrimary}>
                <Text style={[typography.action, styles.disabledActionText]}>Open email app</Text>
              </Pressable>
              <Text style={styles.caption}>Unavailable in this preview</Text>
            </View>
            <View style={styles.footer}>
              <Text accessibilityState={{ disabled: true }} accessibilityLabel="Didn’t get it? Resend unavailable. Password recovery is not connected." style={styles.disabledText}>Didn’t get it? Resend</Text>
              <Text style={styles.caption}>Resend unavailable in this preview</Text>
              <View style={styles.footerActions}>
                <Pressable onPress={onDifferentNic} accessibilityRole="link" accessibilityLabel="Use a different NIC. Return to a fresh Forgot Password screen." style={styles.differentNic}>
                  <Text style={styles.link}>Use a different NIC</Text>
                </Pressable>
              </View>
            </View>
          </View>
          <Text style={styles.country}>SRI LANKA</Text>
        </ScrollView>
      </View>
    </Screen>
  </View>;
}
const styles = StyleSheet.create({
  root: { flex: 1, backgroundColor: '#fff' }, screen: { flex: 1 }, viewport: { flex: 1 }, content: { flexGrow: 1, paddingBottom: 8 },
  topRibbon: { position: 'absolute', left: -9, top: -14 }, skyline: { position: 'absolute', left: -164, bottom: -6, opacity: 0.45 }, bottomRibbon: { position: 'absolute', right: -7, bottom: -8 },
  previewNotice: { marginBottom: 12, fontFamily: 'Inter_400Regular', fontSize: 13, lineHeight: 18, color: '#444', textAlign: 'center', flexShrink: 0 },
  header: { alignItems: 'center', width: '100%', flexShrink: 0 },
  back: { position: 'absolute', left: 0, top: 5, width: layout.touchTarget, height: layout.touchTarget, alignItems: 'center', justifyContent: 'center', zIndex: 2 },
  backCircle: { width: 38, height: 38, borderRadius: 9999, backgroundColor: '#fff', alignItems: 'center', justifyContent: 'center', boxShadow: '0px 1px 4px rgba(0,0,0,0.04)' },
  backIcon: { width: 20, height: 20 }, logoSlot: { marginTop: 6, overflow: 'hidden' },
  vision: { position: 'absolute', right: 9, width: 86, alignItems: 'flex-end', gap: 6 },
  visionText: { fontFamily: 'Inter_500Medium', color: '#666', fontSize: 9, lineHeight: 13, letterSpacing: 2, textAlign: 'right' }, accent: { width: 36, height: 2, backgroundColor: '#f5a915' },
  card: { width: '100%', flexShrink: 0, paddingHorizontal: 20, paddingVertical: 20, gap: 12, alignItems: 'center', borderRadius: 24, borderWidth: 1, borderColor: '#f1f5f9', backgroundColor: '#fff', boxShadow: '0px 10px 22px rgba(0,0,0,0.1)' },
  badge: { width: 72, height: 72, borderRadius: 9999, backgroundColor: '#f1f7fc', alignItems: 'center', justifyContent: 'center' }, mailIcon: { width: 34, height: 34 },
  cardHeader: { width: '100%', gap: 4, alignItems: 'center' }, heading: { fontFamily: 'Inter_700Bold', fontSize: 20, lineHeight: 28, color: '#142033', textAlign: 'center' },
  description: { width: '100%', fontFamily: 'Inter_400Regular', fontSize: 13, lineHeight: 18, color: '#666', textAlign: 'center' },
  sampleEmail: { width: '100%', fontFamily: 'Inter_700Bold', fontSize: 15, lineHeight: 21, color: '#142033', textAlign: 'center' },
  caption: { fontFamily: 'Inter_400Regular', fontSize: 11, lineHeight: 16, color: '#666', textAlign: 'center' },
  actionGroup: { width: '100%', gap: 4 }, disabledPrimary: { width: '100%', minHeight: layout.touchTarget, paddingHorizontal: 24, paddingVertical: 12, alignItems: 'center', justifyContent: 'center', borderRadius: 14, backgroundColor: '#8b1730' },
  disabledActionText: { fontFamily: 'Inter_600SemiBold', color: '#fff', textAlign: 'center' },
  footer: { width: '100%', gap: 4 },
  footerActions: { flexDirection: 'row', flexWrap: 'wrap', justifyContent: 'center', columnGap: 12 },
  disabledText: { fontFamily: 'Inter_500Medium', fontSize: 13, lineHeight: 18, color: '#666', textAlign: 'center' },
  differentNic: { maxWidth: '100%', minHeight: layout.touchTarget, minWidth: layout.touchTarget, justifyContent: 'center' },
  link: { fontFamily: 'Inter_500Medium', fontSize: 13, lineHeight: 18, color: '#a30f2d', textAlign: 'center' },
  country: { marginTop: 'auto', paddingTop: 24, fontFamily: 'Inter_400Regular', fontSize: 9, lineHeight: 13, letterSpacing: 4, color: '#666', textAlign: 'center' },
});
