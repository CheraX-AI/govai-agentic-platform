import { StatusBar } from 'expo-status-bar';
import { OnboardingProgress } from '../components/OnboardingProgress';
import { useState } from 'react';
import { Image, Pressable, ScrollView, StyleSheet, Text, View, useWindowDimensions } from 'react-native';
import { useSafeAreaInsets } from 'react-native-safe-area-context';
import { layout, typography } from '../layout';

export const discoverAssets = {
  identity: require('../assets/figma/onboarding-discover/identity.png'),
  vehicles: require('../assets/figma/onboarding-discover/vehicles.png'),
  certificates: require('../assets/figma/onboarding-discover/certificates.png'),
  passports: require('../assets/figma/onboarding-discover/passports.png'),
  hero: require('../assets/figma/onboarding-discover/hero.jpg'),
  back: require('../assets/figma/onboarding-meet/back.png'),
  next: require('../assets/figma/onboarding-meet/next.png'),
};

export function OnboardingDiscoverScreen({ onBack, onNext, onLogin }: { onBack: () => void; onNext: () => void; onLogin: () => void }) {
  const { fontScale } = useWindowDimensions();
  const insets = useSafeAreaInsets();
  const [viewport, setViewport] = useState({ width: 0, height: 0 });
  const [panelHeight, setPanelHeight] = useState(0);
  // Artwork is full-bleed; controls and panel content own their safe areas locally.
  const availableHeight = viewport.height;
  const contentWidth = Math.min(layout.contentWidth, Math.max(0, viewport.width - insets.left - insets.right));
  const tileSpace = Math.max(0, contentWidth - 32);
  const minimumTileWidth = 86 * Math.max(1, fontScale);
  const columns = tileSpace >= minimumTileWidth * 4 + 24 ? 4
    : tileSpace >= minimumTileWidth * 2 + 8 ? 2 : 1;
  const tileWidth = Math.max(0, (tileSpace - (columns - 1) * 8) / columns);
  const overlap = 24;
  const artworkWidth = viewport.width;
  const artworkHeight = artworkWidth * 1597 / 985;
  // Conservative source-image bounds include the crest, GovAI wordmark and tagline.
  // Keep the whole band (source y=200..630) clear of system UI and the panel.
  const logoTop = artworkWidth * 200 / 985;
  const logoBottom = artworkWidth * 630 / 985;
  const minimumHeroHeight = insets.top + 12 + (logoBottom - logoTop) + overlap + 12;
  const heroHeight = panelHeight > 0 ? Math.max(minimumHeroHeight, availableHeight - panelHeight + overlap)
    : Math.max(minimumHeroHeight, artworkWidth * 382 / 400);
  const preferredImageTop = -0.0509 * artworkWidth * 382 / 400;
  const imageTop = Math.max(insets.top + 12 - logoTop,
    Math.min(preferredImageTop, heroHeight - overlap - 12 - logoBottom));
  const tiles = [
    { title: "NIC & identity", description: "NIC and ID services", icon: discoverAssets.identity, color: '#fcf3f5' },
    { title: "Vehicles", description: "Registration and licences", icon: discoverAssets.vehicles, color: '#f2f8f4' },
    { title: "Certificates", description: "Birth, death and more", icon: discoverAssets.certificates, color: '#f1f7fc' },
    { title: "Passports", description: "Passports and visas", icon: discoverAssets.passports, color: '#fff8ea' },
  ];
  return (
    <View style={styles.root} onLayout={({ nativeEvent: { layout: measured } }) => {
      setViewport(previous => previous.width === measured.width && previous.height === measured.height
        ? previous : { width: measured.width, height: measured.height });
    }}>
      <StatusBar style="dark" />
      <ScrollView style={styles.scroll} contentContainerStyle={styles.content}>
        <View style={[styles.hero, { height: heroHeight }]}>
          <View style={styles.heroCrop}><Image source={discoverAssets.hero} resizeMode="contain" accessible={false}
            style={[styles.heroImage, { width: artworkWidth, height: artworkHeight, top: imageTop }]} /></View>
          <Pressable onPress={onLogin} accessibilityRole="button" accessibilityLabel={"Skip onboarding and open Login"} style={[styles.skip, { top: insets.top + 12, right: insets.right + 16 }]}>
            <Text style={styles.skipText}>{"Skip"}</Text>
          </Pressable>
        </View>
        {/* A static panel: the handle and indicators are decorative, not gestures. */}
        <View style={[styles.panel, { marginTop: -overlap, paddingLeft: insets.left + 16,
          paddingRight: insets.right + 16, paddingBottom: insets.bottom + 12 }]} onLayout={({ nativeEvent: { layout: measured } }) => {
          setPanelHeight(previous => Math.abs(previous - measured.height) < 0.5 ? previous : measured.height);
        }}>
          <View style={styles.panelContent}>
          <View accessible={false} importantForAccessibility="no-hide-descendants" style={styles.handle} />
          <Text accessibilityRole="header" style={[typography.heading, styles.title]}>
            {"Discover"}{'\n'}<Text style={styles.brand}>{"government services"}</Text>
          </Text>
          <Text style={[typography.body, styles.description]}>
            {"Find every Sri Lankan government service in one place, with clear steps for each."}{' '}</Text>
          <View style={styles.tiles}>
            {tiles.map(tile => (
              <View key={tile.title} style={[styles.tile, { backgroundColor: tile.color, width: tileWidth }]}>
                <View style={styles.tileIcon}><Image source={tile.icon} accessible={false} style={styles.icon} /></View>
                <Text style={styles.tileTitle}>{tile.title}</Text>
                <Text style={styles.tileDescription}>{tile.description}</Text>
              </View>
            ))}
          </View>
          <View style={styles.footer}>
            <Pressable onPress={onBack} accessibilityRole="button" accessibilityLabel={"Back to Meet GovAI"} style={styles.back}>
              <Image source={discoverAssets.back} accessible={false} style={styles.icon} />
            </Pressable>
            <OnboardingProgress current={2} />
            <Pressable onPress={onNext} accessibilityRole="button" accessibilityLabel={"Next: Ask GovAI"} style={styles.next}>
              <Image source={discoverAssets.next} accessible={false} style={styles.icon} />
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
  panelContent: { width: '100%', maxWidth: layout.contentWidth - 32, alignItems: 'center', gap: 8 },
  handle: { width: 40, height: 4, borderRadius: 9999, backgroundColor: '#e5e7eb' },
  title: { fontFamily: 'Inter_700Bold', textAlign: 'center', color: '#142033' },
  brand: { color: '#a30f2d' },
  description: { fontFamily: 'Inter_400Regular', color: '#666', textAlign: 'center' },
  tiles: { width: '100%', flexDirection: 'row', flexWrap: 'wrap', gap: 8 },
  tile: { maxWidth: '100%', paddingHorizontal: 6, paddingVertical: 8, borderRadius: 14, gap: 6, alignItems: 'center' },
  tileIcon: { width: 40, height: 40, backgroundColor: '#fff', borderRadius: 12, alignItems: 'center', justifyContent: 'center' },
  tileTitle: { fontFamily: 'Inter_600SemiBold', fontSize: 11, lineHeight: 15, color: '#142033', textAlign: 'center', width: '100%' },
  tileDescription: { fontFamily: 'Inter_400Regular', fontSize: 10, lineHeight: 12, color: '#666', textAlign: 'center', width: '100%' },
  footer: { width: '100%', flexDirection: 'row', alignItems: 'center', justifyContent: 'space-between', gap: 8 },
  back: { width: 52, height: 52, borderRadius: 9999, backgroundColor: '#fcf3f5', alignItems: 'center', justifyContent: 'center' },
  next: { width: 56, height: 56, borderRadius: 9999, backgroundColor: '#a30f2d', alignItems: 'center', justifyContent: 'center', boxShadow: '0px 6px 16px rgba(139,26,46,0.25)' },
  icon: { width: 24, height: 24 },
});

