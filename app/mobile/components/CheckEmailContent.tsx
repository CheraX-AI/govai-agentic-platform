import { useState } from 'react';
import { StatusBar } from 'expo-status-bar';
import { Image, Pressable, ScrollView, StyleSheet, Text, View, useWindowDimensions } from 'react-native';
import { Screen } from './Screen';
import { layout, typography } from '../layout';
import { loginAssets } from '../pages/LoginScreen';
import { registerAssets } from '../pages/RegisterScreen';

const mail = require('../assets/figma/check-email/mail.png');
const info = require('../assets/figma/forgot-password/info.png');

export function CheckEmailContent({ mode, onBack, onDifferentNic, onLogin }: { mode: 'standalone' | 'demo'; onBack: () => void; onDifferentNic: () => void; onLogin: () => void }) {
  if (!__DEV__ || (mode === 'demo' ? process.env.EXPO_PUBLIC_CONNECTED_UI_DEMO !== '1' : process.env.EXPO_PUBLIC_CHECK_EMAIL_PREVIEW !== '1')) return null;
  return <ContentBody onBack={onBack} onDifferentNic={onDifferentNic} onLogin={onLogin} />;
}

function ContentBody({ onBack, onDifferentNic, onLogin }: { onBack: () => void; onDifferentNic: () => void; onLogin: () => void }) {
  const { width, fontScale } = useWindowDimensions();
  const [viewport, setViewport] = useState({ width: 0, height: 0 });
  const [visionHeight, setVisionHeight] = useState(47);
  const [cardHeight, setCardHeight] = useState(0);
  const contentWidth = viewport.width || Math.min(layout.contentWidth, Math.max(0, width - 32));
  const maximumLogoWidth = Math.min(150, width * 0.375, Math.max(0, contentWidth - 2 * (86 + 9 + 4)));
  const maximumLogoHeight = maximumLogoWidth * 151.531 / 150;
  const intrinsicCardHeight = cardHeight || 420 * fontScale;
  const headerGap = Math.max(12, Math.min(16, viewport.height - intrinsicCardHeight - maximumLogoHeight - 31));
  // Keep the original crop and width-safe logo size; short windows scroll.
  const logoHeight = maximumLogoHeight;
  const logoWidth = maximumLogoWidth;
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
          <View style={[styles.card, { marginTop: headerGap }]} onLayout={event => {
            // Intrinsic card height is independent of the flexible decorative header.
            const measured = event.nativeEvent.layout.height; setCardHeight(previous => Math.abs(previous - measured) < 0.5 ? previous : measured);
          }}>
            <View style={styles.badge}><Image source={mail} accessible={false} style={styles.mailIcon} /></View>
            <View style={styles.cardHeader}>
              <Text accessibilityRole="header" style={styles.heading}>Check your email</Text>
              <Text style={styles.description}>Demo preview — no email was sent.</Text>
              <Text style={styles.description}>Sample reset-link destination:</Text>
              <Text style={styles.sampleEmail}>sample@example.test</Text>
            </View>
            <View style={styles.infoPanel}>
              <Image source={info} accessible={false} style={styles.infoIcon} />
              <Text style={styles.infoText}>Check your spam folder or try again.</Text>
            </View>
            <Pressable onPress={onLogin} accessibilityRole="button" accessibilityLabel="Back to log in"
              accessibilityHint="Returns to a fresh Login form. No email was sent." style={styles.primary}>
              <Text style={[typography.action, styles.primaryText]}>Back to log in</Text>
              <Image source={registerAssets.next} accessible={false} style={styles.nextIcon} />
            </Pressable>
            <View style={styles.footer}>
              <View style={styles.resendRow}>
                <Text style={styles.resendPrompt}>Didn’t get it?</Text>
                <Pressable disabled accessibilityRole="link" accessibilityState={{ disabled: true }}
                  accessibilityLabel="Resend link. Unavailable in demo; password recovery is not connected." style={styles.resendLink}>
                  <Text style={styles.link}>Resend link</Text>
                </Pressable>
              </View>
              <Text style={styles.caption}>Unavailable in demo</Text>
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
  header: { alignItems: 'center', width: '100%', flexShrink: 0 },
  back: { position: 'absolute', left: 0, top: 5, width: layout.touchTarget, height: layout.touchTarget, alignItems: 'center', justifyContent: 'center', zIndex: 2 },
  backCircle: { width: 38, height: 38, borderRadius: 9999, backgroundColor: '#fff', alignItems: 'center', justifyContent: 'center', boxShadow: '0px 1px 4px rgba(0,0,0,0.04)' },
  backIcon: { width: 20, height: 20 }, logoSlot: { marginTop: 6, overflow: 'hidden' },
  vision: { position: 'absolute', right: 9, width: 86, alignItems: 'flex-end', gap: 6 },
  visionText: { fontFamily: 'Inter_500Medium', color: '#666', fontSize: 9, lineHeight: 13, letterSpacing: 2, textAlign: 'right' }, accent: { width: 36, height: 2, backgroundColor: '#f5a915' },
  card: { width: '100%', flexShrink: 0, paddingHorizontal: 16, paddingVertical: 16, gap: 8, alignItems: 'center', borderRadius: 24, borderWidth: 1, borderColor: '#f1f5f9', backgroundColor: '#fff', boxShadow: '0px 10px 22px rgba(0,0,0,0.1)' },
  badge: { width: 64, height: 64, borderRadius: 9999, backgroundColor: '#edf6f1', alignItems: 'center', justifyContent: 'center' }, mailIcon: { width: 32, height: 32, tintColor: '#007a53' },
  cardHeader: { width: '100%', gap: 4, alignItems: 'center' }, heading: { fontFamily: 'Inter_700Bold', fontSize: 20, lineHeight: 28, color: '#142033', textAlign: 'center' },
  description: { width: '100%', fontFamily: 'Inter_400Regular', fontSize: 13, lineHeight: 18, color: '#666', textAlign: 'center' },
  sampleEmail: { width: '100%', fontFamily: 'Inter_700Bold', fontSize: 15, lineHeight: 21, color: '#142033', textAlign: 'center' },
  caption: { fontFamily: 'Inter_400Regular', fontSize: 11, lineHeight: 16, color: '#666', textAlign: 'center' },
  infoPanel: { width: '100%', flexDirection: 'row', alignItems: 'center', gap: 12, padding: 12, borderRadius: 12, backgroundColor: '#eaf4ff' },
  infoIcon: { width: 20, height: 20 }, infoText: { flex: 1, fontFamily: 'Inter_400Regular', fontSize: 13, lineHeight: 18, color: '#123965' },
  primary: { width: '100%', minHeight: layout.touchTarget, paddingHorizontal: 20, paddingVertical: 12, flexDirection: 'row', flexWrap: 'wrap', gap: 10, alignItems: 'center', justifyContent: 'center', borderRadius: 14, backgroundColor: '#8b1730', boxShadow: '0px 6px 16px rgba(139,23,48,0.2)' },
  primaryText: { flexShrink: 1, fontFamily: 'Inter_600SemiBold', color: '#fff', textAlign: 'center' }, nextIcon: { width: 18, height: 18 },
  footer: { width: '100%', gap: 2 },
  resendRow: { flexDirection: 'row', flexWrap: 'wrap', alignItems: 'center', justifyContent: 'center', columnGap: 4 },
  resendPrompt: { fontFamily: 'Inter_400Regular', fontSize: 13, lineHeight: 18, color: '#666', textAlign: 'center' },
  resendLink: { minHeight: layout.touchTarget, minWidth: layout.touchTarget, justifyContent: 'center' },
  footerActions: { flexDirection: 'row', flexWrap: 'wrap', justifyContent: 'center', columnGap: 12 },
  differentNic: { maxWidth: '100%', minHeight: layout.touchTarget, minWidth: layout.touchTarget, justifyContent: 'center' },
  link: { fontFamily: 'Inter_500Medium', fontSize: 13, lineHeight: 18, color: '#a30f2d', textAlign: 'center', textDecorationLine: 'underline' },
  country: { marginTop: 'auto', paddingTop: 4, fontFamily: 'Inter_400Regular', fontSize: 9, lineHeight: 13, letterSpacing: 4, color: '#666', textAlign: 'center' },
});
