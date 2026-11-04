import { StatusBar } from 'expo-status-bar';
import { OnboardingProgress } from './components/OnboardingProgress';
import { useState } from 'react';
import { Image, Pressable, ScrollView, StyleSheet, Text, View } from 'react-native';
import { useSafeAreaInsets } from 'react-native-safe-area-context';
import { layout, typography } from './layout';

export const askAssets = {
  hero: require('./assets/figma/onboarding-discover/hero.jpg'),
  back: require('./assets/figma/onboarding-meet/back.png'),
  next: require('./assets/figma/onboarding-meet/next.png'),
};

export function OnboardingAskScreen({ onBack, onNext, onLogin }: { onBack: () => void; onNext: () => void; onLogin: () => void }) {
  const insets = useSafeAreaInsets();
  const [viewport, setViewport] = useState({ width: 0, height: 0 });
  const [bodyHeight, setBodyHeight] = useState(0);
  const [footerHeight, setFooterHeight] = useState(56);
  // Map Figma's 400 x 772 display into the native safe-content coordinates.
  // Its simulated system bands are 30 top / 30 bottom: panel y=340 becomes
  // y=310 in the 712-high safe region. Reinsert the actual top inset once.
  const safeHeight = Math.max(0, viewport.height - insets.top - insets.bottom);
  const preferredPanelTop = insets.top + safeHeight * (340 - 30) / (772 - 60);
  const overlap = 42 * Math.min(viewport.width / 400, safeHeight / 712);
  const artworkWidth = viewport.width;
  const artworkHeight = artworkWidth * 1597 / 985;
  // Conservative source-image bounds include the crest, GovAI wordmark and tagline.
  // Keep the whole band (source y=200..630) clear of system UI and the panel.
  const logoTop = artworkWidth * 200 / 985;
  const logoBottom = artworkWidth * 630 / 985;
  const minimumHeroHeight = insets.top + 12 + (logoBottom - logoTop) + overlap + 12;
  // Only intrinsic body/footer sizes enter this budget; flexible white space
  // is never measured back into the hero calculation.
  const naturalPanelHeight = 10 + bodyHeight + 16 + footerHeight + insets.bottom + 12;
  const preferredHeroHeight = preferredPanelTop + overlap;
  const heroHeight = Math.max(minimumHeroHeight, bodyHeight > 0
    ? Math.min(preferredHeroHeight, viewport.height - naturalPanelHeight + overlap)
    : preferredHeroHeight);
  const preferredImageTop = -0.0509 * artworkWidth * 382 / 400;
  const imageTop = Math.max(insets.top + 12 - logoTop,
    Math.min(preferredImageTop, heroHeight - overlap - 12 - logoBottom));
  return (
    <View style={styles.root} onLayout={({ nativeEvent: { layout: measured } }) => {
      setViewport(previous => previous.width === measured.width && previous.height === measured.height
        ? previous : { width: measured.width, height: measured.height });
    }}>
      <StatusBar style="dark" />
      <ScrollView style={styles.scroll} contentContainerStyle={styles.content}>
        <View style={[styles.hero, { height: heroHeight }]}>
          <View style={styles.heroCrop}><Image source={askAssets.hero} resizeMode="contain" accessible={false}
            style={[styles.heroImage, { width: artworkWidth, height: artworkHeight, top: imageTop }]} /></View>
          <Pressable onPress={onLogin} accessibilityRole="button" accessibilityLabel={"Skip onboarding and open Login"} style={[styles.skip, { top: insets.top + 12, right: insets.right + 16 }]}>
            <Text style={styles.skipText}>{"Skip"}</Text>
          </Pressable>
        </View>
        {/* A static panel: the handle and indicators are decorative, not gestures. */}
        <View style={[styles.panel, { marginTop: -overlap, paddingLeft: insets.left + 16,
          paddingRight: insets.right + 16, paddingBottom: insets.bottom + 12,
          minHeight: Math.max(0, viewport.height - heroHeight + overlap) }]}>
          <View style={styles.panelContent}>
          <View style={styles.panelBody} onLayout={({ nativeEvent: { layout: measured } }) => {
            setBodyHeight(previous => Math.abs(previous - measured.height) < 0.5 ? previous : measured.height);
          }}>
          <View accessible={false} importantForAccessibility="no-hide-descendants" style={styles.handle} />
          <Text accessibilityRole="header" style={[typography.heading, styles.title]}>
            {"Ask GovAI"}{'\n'}<Text style={styles.brand}>{"in your language"}</Text>
          </Text>
          <Text style={[typography.body, styles.description]}>
            {"Get answers in English, Sinhala or Tamil, with step-by-step help for every service."}{' '}</Text>
          {/* Static onboarding example, not a functional chat. */}
          <View style={styles.conversation}>
            <View style={styles.userBubble}>
              <Text style={styles.userText}>{"How do I renew my NIC?"}</Text>
            </View>
            <View style={styles.aiBubble}>
              <Text style={styles.aiText}>{"Log in to the DRP portal, fill in the renewal form and upload your photo. I’ll guide you through each step."}</Text>
            </View>
            <View style={styles.languages}>
              <View style={[styles.chip, styles.englishChip]}><Text style={styles.englishText}>English</Text></View>
              <View style={styles.chip}><Text style={styles.sinhalaText}>සිංහල</Text></View>
              <View style={styles.chip}><Text style={styles.tamilText}>தமிழ்</Text></View>
            </View>
          </View>
          </View>
          <View style={styles.panelSpacer} />
          <View style={styles.footer} onLayout={({ nativeEvent: { layout: measured } }) => {
            setFooterHeight(previous => Math.abs(previous - measured.height) < 0.5 ? previous : measured.height);
          }}>
            <Pressable onPress={onBack} accessibilityRole="button" accessibilityLabel={"Back to Discover"} style={styles.back}>
              <Image source={askAssets.back} accessible={false} style={styles.icon} />
            </Pressable>
            <OnboardingProgress current={3} />
            <Pressable onPress={onNext} accessibilityRole="button" accessibilityLabel={"Next: Track every request"} style={styles.next}>
              <Image source={askAssets.next} accessible={false} style={styles.icon} />
            </Pressable>
          </View>
          </View>
        </View>
      </ScrollView>
    </View>
  );
}

const styles = StyleSheet.create({
  root: { flex: 1, backgroundColor: '#f8fafc' },
  scroll: { flex: 1 },
  content: { flexGrow: 1 },
  hero: { width: '100%', flexShrink: 0, alignItems: 'center', justifyContent: 'flex-end' },
  heroCrop: { ...StyleSheet.absoluteFill, overflow: 'hidden' },
  heroImage: { position: 'absolute', left: 0 },
  skip: { position: 'absolute', right: 16, top: 0, minWidth: layout.touchTarget, minHeight: layout.touchTarget,
    paddingHorizontal: 14, justifyContent: 'center', borderRadius: 9999, backgroundColor: '#ffffffeb' },
  skipText: { fontFamily: 'Inter_500Medium', fontSize: 13, lineHeight: 18, color: '#a30f2d' },
  panel: { flexShrink: 0, paddingTop: 10, paddingBottom: 12,
    paddingHorizontal: 16, borderTopLeftRadius: 24, borderTopRightRadius: 24,
    backgroundColor: '#fff', alignItems: 'center' },
  panelContent: { flexGrow: 1, width: '100%', maxWidth: layout.contentWidth - 32, alignItems: 'center', gap: 8 },
  panelBody: { flexShrink: 0, width: '100%', alignItems: 'center', gap: 8 },
  panelSpacer: { flexGrow: 1 },
  handle: { width: 40, height: 4, borderRadius: 9999, backgroundColor: '#e5e7eb' },
  title: { fontFamily: 'Inter_700Bold', textAlign: 'center', color: '#142033' },
  brand: { color: '#a30f2d' },
  description: { fontFamily: 'Inter_400Regular', color: '#666', textAlign: 'center' },
  conversation: { width: '100%', alignItems: 'center', gap: 8 },
  userBubble: { maxWidth: '100%', paddingHorizontal: 14, paddingVertical: 10, backgroundColor: '#a30f2d', borderRadius: 16, borderBottomRightRadius: 2 },
  userText: { fontFamily: 'Inter_400Regular', fontSize: 13, lineHeight: 18, color: '#fff' },
  aiBubble: { width: '100%', maxWidth: 298, paddingHorizontal: 14, paddingVertical: 10, backgroundColor: '#f1f7fc', borderRadius: 16, borderTopLeftRadius: 2 },
  aiText: { fontFamily: 'Inter_400Regular', fontSize: 12, lineHeight: 17, color: '#142033' },
  languages: { flexDirection: 'row', flexWrap: 'wrap', justifyContent: 'center', gap: 6, width: '100%' },
  chip: { maxWidth: '100%', paddingHorizontal: 10, paddingVertical: 6, borderRadius: 20, backgroundColor: '#f5f5f5', justifyContent: 'center' },
  englishChip: { backgroundColor: '#a30f2d' },
  englishText: { fontFamily: 'Inter_600SemiBold', fontSize: 12, lineHeight: 18, color: '#fff' },
  sinhalaText: { fontFamily: 'Yaldevi_400Regular', fontSize: 14, color: '#444' },
  tamilText: { fontFamily: 'NotoSansTamil_400Regular', fontSize: 14, color: '#444' },
  footer: { flexShrink: 0, width: '100%', flexDirection: 'row', alignItems: 'center', justifyContent: 'space-between', gap: 8 },
  back: { width: 52, height: 52, borderRadius: 9999, backgroundColor: '#fcf3f5', alignItems: 'center', justifyContent: 'center' },
  next: { width: 56, height: 56, borderRadius: 9999, backgroundColor: '#a30f2d', alignItems: 'center', justifyContent: 'center', boxShadow: '0px 6px 16px rgba(139,26,46,0.25)' },
  icon: { width: 24, height: 24 },
});

