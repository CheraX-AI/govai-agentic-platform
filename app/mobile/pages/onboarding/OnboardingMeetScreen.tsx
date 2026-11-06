import { StatusBar } from 'expo-status-bar';
import { Image, Pressable, StyleSheet, Text, View } from 'react-native';

import { OnboardingProgress } from '../../components/onboarding/OnboardingProgress';
import { Screen } from '../../components/common/Screen';
import { layout, typography } from '../../layout';

export const onboardingAssets = {
  hero: require('../../assets/figma/onboarding-meet/hero.png'),
  back: require('../../assets/figma/onboarding-meet/back.png'),
  next: require('../../assets/figma/onboarding-meet/next.png'),
};

export function OnboardingMeetScreen({
  onBack,
  onNext,
  onLogin,
}: {
  onBack: () => void;
  onNext: () => void;
  onLogin: () => void;
}) {
  return (
    <View style={styles.root}>
      <StatusBar style="dark" />
      <Screen horizontalPadding={0} contentStyle={styles.content}>
        <View style={styles.hero}>
          <Image
            source={onboardingAssets.hero}
            resizeMode="contain"
            accessible={false}
            style={[StyleSheet.absoluteFill, { width: '100%', height: '100%' }]}
          />
          <Pressable
            onPress={onLogin}
            accessibilityRole="button"
            accessibilityLabel={'Skip onboarding and open Login'}
            style={styles.skip}
          >
            <Text style={styles.skipText}>{'Skip'}</Text>
          </Pressable>
        </View>
        {/* A static panel: the handle and indicators are decorative, not gestures. */}

        <View style={styles.panel}>
          <View
            accessible={false}
            importantForAccessibility="no-hide-descendants"
            style={styles.handle}
          />
          <Text accessibilityRole="header" style={[typography.heading, styles.title]}>
            {'Meet GovAI'}
            {'\n'}
            <Text style={styles.brand}>{'your government guide'}</Text>
          </Text>
          <Text style={[typography.body, styles.description]}>
            {
              'I explain services, tell you which documents you need and help you track requests, in Sinhala, Tamil or English.'
            }{' '}
          </Text>
          <View style={styles.spacer} />
          <View style={styles.footer}>
            <Pressable
              onPress={onBack}
              accessibilityRole="button"
              accessibilityLabel={'Back to Welcome'}
              style={styles.back}
            >
              <Image source={onboardingAssets.back} accessible={false} style={styles.icon} />
            </Pressable>
            <OnboardingProgress current={1} />
            <Pressable
              onPress={onNext}
              accessibilityRole="button"
              accessibilityLabel={'Next: Discover'}
              style={styles.next}
            >
              <Image source={onboardingAssets.next} accessible={false} style={styles.icon} />
            </Pressable>
          </View>
        </View>
      </Screen>
    </View>
  );
}

const styles = StyleSheet.create({
  root: { flex: 1, backgroundColor: '#f8fafc' },
  content: { backgroundColor: '#f8fafc' },
  hero: { width: '100%', aspectRatio: 400 / 372 },
  skip: {
    alignSelf: 'flex-end',
    minWidth: layout.touchTarget,
    minHeight: layout.touchTarget,
    paddingHorizontal: 14,
    justifyContent: 'center',
    borderRadius: 9999,
    backgroundColor: '#ffffffeb',
  },
  skipText: { fontFamily: 'Inter_500Medium', fontSize: 13, lineHeight: 18, color: '#a30f2d' },
  panel: {
    marginTop: -12,
    flexGrow: 1,
    minHeight: 368,
    paddingTop: 10,
    paddingBottom: 32,
    paddingHorizontal: 16,
    borderTopLeftRadius: 24,
    borderTopRightRadius: 24,
    backgroundColor: '#fff',
    alignItems: 'center',
    gap: 12,
  },
  handle: { width: 40, height: 4, borderRadius: 9999, backgroundColor: '#e5e7eb' },
  title: { fontFamily: 'Inter_700Bold', textAlign: 'center', color: '#142033' },
  brand: { color: '#a30f2d' },
  description: { fontFamily: 'Inter_400Regular', color: '#666', textAlign: 'center' },
  spacer: { flexGrow: 1, minHeight: 12 },
  footer: {
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
