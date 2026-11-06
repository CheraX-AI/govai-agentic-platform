import { StatusBar } from 'expo-status-bar';
import { useState } from 'react';
import {
  Image,
  Pressable,
  ScrollView,
  StyleSheet,
  Text,
  View,
  useWindowDimensions,
} from 'react-native';
import { useSafeAreaInsets } from 'react-native-safe-area-context';

import { OnboardingProgress } from '../../components/onboarding/OnboardingProgress';
import { layout, typography } from '../../layout';

export const trackAssets = {
  identity: require('../../assets/figma/onboarding-track/identity.png'),
  calendar: require('../../assets/figma/onboarding-track/calendar.png'),
  clock: require('../../assets/figma/onboarding-track/clock.png'),
  chevron: require('../../assets/figma/onboarding-track/chevron.png'),
  complete: require('../../assets/figma/onboarding-track/complete.png'),
  current: require('../../assets/figma/onboarding-track/current.png'),
  pending: require('../../assets/figma/onboarding-track/pending.png'),
  hero: require('../../assets/figma/onboarding-discover/hero.jpg'),
  back: require('../../assets/figma/onboarding-meet/back.png'),
  next: require('../../assets/figma/onboarding-meet/next.png'),
};

// Fit the static tracking example around text growth and safe areas.

export function OnboardingTrackScreen({
  onBack,
  onNext,
  onLogin,
}: {
  onBack: () => void;
  onNext: () => void;
  onLogin: () => void;
}) {
  const insets = useSafeAreaInsets();
  const { fontScale } = useWindowDimensions();

  const [viewport, setViewport] = useState({ width: 0, height: 0 });
  const compactCard =
    Math.min(layout.contentWidth - 32, viewport.width - insets.left - insets.right - 32) <
    340 * fontScale;
  const stages = [
    { label: 'Application Submitted', status: 'Complete', icon: trackAssets.complete },
    { label: 'Documents Verified', status: 'Complete', icon: trackAssets.complete },
    { label: 'Under Review', status: 'Current', icon: trackAssets.current },
    { label: 'Approval', status: 'Pending', icon: trackAssets.pending },
    { label: 'Completed', status: 'Pending', icon: trackAssets.pending },
  ];
  const [bodyHeight, setBodyHeight] = useState(0);
  const [footerHeight, setFooterHeight] = useState(56);
  // Map Figma's 400 x 772 display into the native safe-content coordinates.
  // Its simulated system bands are 30 top / 30 bottom: panel y=340 becomes
  // y=310 in the 712-high safe region. Reinsert the actual top inset once.

  const safeHeight = Math.max(0, viewport.height - insets.top - insets.bottom);
  const preferredPanelTop = insets.top + (safeHeight * (340 - 30)) / (772 - 60);
  const overlap = 42 * Math.min(viewport.width / 400, safeHeight / 712);
  const artworkWidth = viewport.width;
  const artworkHeight = (artworkWidth * 1597) / 985;
  // Conservative source-image bounds include the crest, GovAI wordmark and tagline.
  // Keep the whole band (source y=200..630) clear of system UI and the panel.

  const logoTop = (artworkWidth * 200) / 985;
  const logoBottom = (artworkWidth * 630) / 985;
  const minimumHeroHeight = insets.top + 12 + (logoBottom - logoTop) + overlap + 12;
  // Only intrinsic body/footer sizes enter this budget; flexible white space
  // is never measured back into the hero calculation.

  const naturalPanelHeight = 10 + bodyHeight + 16 + footerHeight + insets.bottom + 12;
  const preferredHeroHeight = preferredPanelTop + overlap;
  const heroHeight = Math.max(
    minimumHeroHeight,
    bodyHeight > 0
      ? Math.min(preferredHeroHeight, viewport.height - naturalPanelHeight + overlap)
      : preferredHeroHeight,
  );
  const preferredImageTop = (-0.0509 * artworkWidth * 382) / 400;
  const imageTop = Math.max(
    insets.top + 12 - logoTop,
    Math.min(preferredImageTop, heroHeight - overlap - 12 - logoBottom),
  );
  return (
    <View
      style={styles.root}
      onLayout={({ nativeEvent: { layout: measured } }) => {
        setViewport(previous =>
          previous.width === measured.width && previous.height === measured.height
            ? previous
            : { width: measured.width, height: measured.height },
        );
      }}
    >
      <StatusBar style="dark" />
      <ScrollView style={styles.scroll} contentContainerStyle={styles.content}>
        <View style={[styles.hero, { height: heroHeight }]}>
          <View style={styles.heroCrop}>
            <Image
              source={trackAssets.hero}
              resizeMode="contain"
              accessible={false}
              style={[
                styles.heroImage,
                { width: artworkWidth, height: artworkHeight, top: imageTop },
              ]}
            />
          </View>
          <Pressable
            onPress={onLogin}
            accessibilityRole="button"
            accessibilityLabel={'Skip onboarding and open Login'}
            style={[styles.skip, { top: insets.top + 12, right: insets.right + 16 }]}
          >
            <Text style={styles.skipText}>{'Skip'}</Text>
          </Pressable>
        </View>
        {/* A static panel: the handle and indicators are decorative, not gestures. */}

        <View
          style={[
            styles.panel,
            {
              marginTop: -overlap,
              paddingLeft: insets.left + 16,
              paddingRight: insets.right + 16,
              paddingBottom: insets.bottom + 12,
              minHeight: Math.max(0, viewport.height - heroHeight + overlap),
            },
          ]}
        >
          <View style={styles.panelContent}>
            <View
              style={styles.panelBody}
              onLayout={({ nativeEvent: { layout: measured } }) => {
                setBodyHeight(previous =>
                  Math.abs(previous - measured.height) < 0.5 ? previous : measured.height,
                );
              }}
            >
              <View
                accessible={false}
                importantForAccessibility="no-hide-descendants"
                style={styles.handle}
              />
              <Text accessibilityRole="header" style={[typography.heading, styles.title]}>
                {'Track every'}
                {'\n'}
                <Text style={styles.brand}>{'request'}</Text>
              </Text>
              <Text style={[typography.body, styles.description]}>
                {'See where each application stands and know when you need to act.'}{' '}
              </Text>
              {/* Static example: no card action, request lookup or backend calls. */}

              <View style={styles.requestCard}>
                <View style={[styles.summary, compactCard && styles.summaryWrapped]}>
                  <View style={styles.summaryDetails}>
                    <View style={styles.cardIcon}>
                      <Image source={trackAssets.identity} accessible={false} style={styles.icon} />
                    </View>
                    <View style={styles.summaryText}>
                      <Text style={styles.cardTitle}>{'NIC Renewal'}</Text>
                      <Text style={styles.reference}>{'Ref: GSR/2026/001234'}</Text>
                      <View style={styles.submitted}>
                        <Image
                          source={trackAssets.calendar}
                          accessible={false}
                          style={styles.calendar}
                        />
                        <Text style={styles.submittedText}>{'Submitted on 12 Sep 2026'}</Text>
                      </View>
                    </View>
                  </View>
                  <View style={styles.statusGroup}>
                    <View style={styles.badge}>
                      <Image source={trackAssets.clock} accessible={false} style={styles.clock} />
                      <Text style={styles.badgeText}>{'In Progress'}</Text>
                    </View>
                    <Image source={trackAssets.chevron} accessible={false} style={styles.chevron} />
                  </View>
                </View>
                <View style={[styles.tracker, compactCard && styles.trackerVertical]}>
                  {stages.map((stage, index) => (
                    <View
                      key={stage.label}
                      accessible
                      accessibilityLabel={`${stage.label}. ${stage.status}.`}
                      style={compactCard ? styles.verticalStep : styles.step}
                    >
                      <View style={compactCard ? styles.verticalMarker : styles.track}>
                        {!compactCard && (
                          <View
                            style={[
                              styles.line,
                              index > 0 && { backgroundColor: index <= 2 ? '#10b981' : '#e5e7eb' },
                            ]}
                          />
                        )}
                        <View style={styles.markerSlot}>
                          <Image
                            source={stage.icon}
                            accessible={false}
                            style={index === 2 ? styles.currentMarker : styles.marker}
                          />
                        </View>
                        {!compactCard && (
                          <View
                            style={[
                              styles.line,
                              index < 4 && { backgroundColor: index < 2 ? '#10b981' : '#e5e7eb' },
                            ]}
                          />
                        )}
                      </View>
                      <Text
                        style={[
                          styles.stageText,
                          compactCard && styles.verticalLabel,
                          index > 2 && styles.pendingText,
                        ]}
                      >
                        {stage.label}
                      </Text>
                    </View>
                  ))}
                </View>
              </View>
            </View>
            <View style={styles.panelSpacer} />
            <View
              style={styles.footer}
              onLayout={({ nativeEvent: { layout: measured } }) => {
                setFooterHeight(previous =>
                  Math.abs(previous - measured.height) < 0.5 ? previous : measured.height,
                );
              }}
            >
              <Pressable
                onPress={onBack}
                accessibilityRole="button"
                accessibilityLabel={'Back to Ask'}
                style={styles.back}
              >
                <Image source={trackAssets.back} accessible={false} style={styles.icon} />
              </Pressable>
              <OnboardingProgress current={4} />
              <Pressable
                onPress={onNext}
                accessibilityRole="button"
                accessibilityLabel={'Next: Pay and stay informed'}
                style={styles.next}
              >
                <Image source={trackAssets.next} accessible={false} style={styles.icon} />
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
  skip: {
    position: 'absolute',
    right: 16,
    top: 0,
    minWidth: layout.touchTarget,
    minHeight: layout.touchTarget,
    paddingHorizontal: 14,
    justifyContent: 'center',
    borderRadius: 9999,
    backgroundColor: '#ffffffeb',
  },
  skipText: { fontFamily: 'Inter_500Medium', fontSize: 13, lineHeight: 18, color: '#a30f2d' },
  panel: {
    flexShrink: 0,
    paddingTop: 10,
    paddingBottom: 12,
    paddingHorizontal: 16,
    borderTopLeftRadius: 24,
    borderTopRightRadius: 24,
    backgroundColor: '#fff',
    alignItems: 'center',
  },
  panelContent: {
    flexGrow: 1,
    width: '100%',
    maxWidth: layout.contentWidth - 32,
    alignItems: 'center',
    gap: 8,
  },
  panelBody: { flexShrink: 0, width: '100%', alignItems: 'center', gap: 8 },
  panelSpacer: { flexGrow: 1 },
  handle: { width: 40, height: 4, borderRadius: 9999, backgroundColor: '#e5e7eb' },
  title: { fontFamily: 'Inter_700Bold', textAlign: 'center', color: '#142033' },
  brand: { color: '#a30f2d' },
  description: { fontFamily: 'Inter_400Regular', color: '#666', textAlign: 'center' },
  requestCard: {
    width: '100%',
    borderWidth: 1,
    borderColor: '#e5e7eb',
    borderRadius: 16,
    padding: 12,
    gap: 14,
    backgroundColor: '#fff',
    boxShadow: '0px 2px 8px rgba(0,0,0,0.03)',
  },
  summary: { flexDirection: 'row', alignItems: 'center', gap: 8 },
  summaryWrapped: { flexWrap: 'wrap' },
  summaryDetails: {
    flexDirection: 'row',
    alignItems: 'center',
    gap: 8,
    flexGrow: 1,
    flexShrink: 1,
  },
  summaryText: { flexShrink: 1, gap: 2 },
  cardIcon: {
    width: 40,
    height: 40,
    borderRadius: 12,
    backgroundColor: '#fcf3f5',
    alignItems: 'center',
    justifyContent: 'center',
  },
  cardTitle: { fontFamily: 'Inter_700Bold', fontSize: 14, lineHeight: 19, color: '#142033' },
  reference: { fontFamily: 'Inter_400Regular', fontSize: 11, lineHeight: 15, color: '#666' },
  submitted: { flexDirection: 'row', alignItems: 'center', gap: 4 },
  calendar: { width: 12, height: 12 },
  submittedText: {
    flexShrink: 1,
    fontFamily: 'Inter_400Regular',
    fontSize: 10,
    lineHeight: 12,
    color: '#666666',
  },
  statusGroup: { flexDirection: 'row', flexShrink: 1, alignItems: 'center', gap: 8 },
  badge: {
    flexDirection: 'row',
    flexShrink: 1,
    alignItems: 'center',
    gap: 6,
    paddingHorizontal: 10,
    paddingVertical: 6,
    borderRadius: 9999,
    backgroundColor: '#fcf3f5',
  },
  clock: { width: 14, height: 14 },
  badgeText: {
    flexShrink: 1,
    fontFamily: 'Inter_600SemiBold',
    fontSize: 12,
    lineHeight: 16,
    color: '#a30f2d',
  },
  chevron: { width: 16, height: 16 },
  tracker: { flexDirection: 'row', width: '100%' },
  trackerVertical: { flexDirection: 'column', gap: 8 },
  step: { flex: 1, alignItems: 'center', gap: 6 },
  track: { flexDirection: 'row', width: '100%', height: 18, alignItems: 'center' },
  line: { flex: 1, height: 2 },
  markerSlot: { width: 12, height: 18, alignItems: 'center', justifyContent: 'center' },
  marker: { width: 8, height: 8 },
  currentMarker: { width: 18, height: 18 },
  verticalStep: { flexDirection: 'row', alignItems: 'center', gap: 8 },
  verticalMarker: { width: 18, alignItems: 'center' },
  stageText: {
    width: '100%',
    fontFamily: 'Inter_500Medium',
    fontSize: 9,
    lineHeight: 12,
    color: '#444',
    textAlign: 'center',
  },
  verticalLabel: { width: 'auto', flex: 1, textAlign: 'left' },
  pendingText: { color: '#666666' },
  footer: {
    flexShrink: 0,
    width: '100%',
    flexDirection: 'row',
    alignItems: 'center',
    justifyContent: 'space-between',
    gap: 8,
  },
  back: {
    width: 52,
    height: 52,
    borderRadius: 9999,
    backgroundColor: '#fcf3f5',
    alignItems: 'center',
    justifyContent: 'center',
  },
  next: {
    width: 56,
    height: 56,
    borderRadius: 9999,
    backgroundColor: '#a30f2d',
    alignItems: 'center',
    justifyContent: 'center',
    boxShadow: '0px 6px 16px rgba(139,26,46,0.25)',
  },
  icon: { width: 24, height: 24 },
});
