import { StatusBar } from 'expo-status-bar';
import { Image, Pressable, StyleSheet, Text, View } from 'react-native';
import { Screen } from './components/Screen';
import { layout, typography } from './layout';

export const welcomeAssets = {
  hero: require('./assets/figma/welcome/hero.jpg'),
  next: require('./assets/figma/welcome/next.png'),
  globe: require('./assets/figma/welcome/globe.png'),
  chevron: require('./assets/figma/welcome/chevron.png'),
};

export function WelcomeScreen({ onGetStarted, onLogin }: { onGetStarted: () => void; onLogin: () => void }) {
  return (
    <View style={styles.root}>
      <StatusBar style="dark" />
      <Image source={welcomeAssets.hero} fadeDuration={0} resizeMode="cover" accessible={false} style={[StyleSheet.absoluteFill, { width: '100%', height: '100%' }]} />
      <View pointerEvents="none" style={[styles.scrim, { top: '38.86%' }]} />
      <Screen>
        <Pressable disabled accessibilityRole="button" accessibilityLabel="Language: English. Language selection unavailable." accessibilityState={{ disabled: true }} style={styles.language}>
          <View style={styles.languagePill}>
          <Image source={welcomeAssets.globe} fadeDuration={0} accessible={false} style={styles.icon} />
          <Text style={styles.languageText}>EN</Text>
          <Image source={welcomeAssets.chevron} fadeDuration={0} accessible={false} style={styles.chevron} />
          </View>
        </Pressable>
        <View style={styles.intro}>
          <Text accessibilityRole="header" style={[styles.heading, typography.heading]}>{"Government services, simplified"}</Text>
          <Text style={[styles.description, typography.body]}>{"Apply, pay and track your requests in one app, with GovAI to guide you."}</Text>
          <View style={{ height: 8 }} />
          <Pressable onPress={onGetStarted} accessibilityRole="button" accessibilityLabel={"Get started"} style={styles.start}>
            <Text style={[styles.startText, typography.action]}>{"Get started"}</Text>
            <Image source={welcomeAssets.next} fadeDuration={0} accessible={false} style={styles.icon} />
          </Pressable>
          <Pressable onPress={onLogin} accessibilityRole="link" accessibilityLabel={"Already have an account? Log in"} style={styles.login}>
            <Text style={styles.loginText}>{"Already have an account?"}{' '}<Text style={styles.loginAccent}>{"Log in"}</Text></Text>
          </Pressable>
        </View>
      </Screen>
    </View>
  );
}

const styles = StyleSheet.create({
  root: { flex: 1, backgroundColor: '#142033' },
  scrim: { position: 'absolute', left: 0, right: 0, bottom: 0, experimental_backgroundImage: 'linear-gradient(to bottom, rgba(0,0,0,0) 0%, rgba(0,0,0,0.55) 50%, rgba(0,0,0,0.8) 100%)' },
  language: { alignSelf: 'flex-end', minWidth: layout.touchTarget, minHeight: layout.touchTarget, justifyContent: 'center', alignItems: 'center' },
  languagePill: { flexDirection: 'row', alignItems: 'center', gap: 4, paddingHorizontal: 10, paddingVertical: 6, borderRadius: 9999, backgroundColor: '#fff' },
  languageText: { fontFamily: 'Inter_600SemiBold', fontSize: 11, lineHeight: 13, color: '#142033' },
  icon: { width: 16, height: 16 },
  chevron: { width: 14, height: 14 },
  intro: { marginTop: 'auto', gap: 12, alignItems: 'center', width: '100%' },
  heading: { fontFamily: 'Inter_700Bold', color: '#fff', textAlign: 'center', includeFontPadding: false },
  description: { fontFamily: 'Inter_400Regular', color: '#fff', textAlign: 'center', includeFontPadding: false },
  start: { width: '100%', minHeight: layout.touchTarget, paddingVertical: 16, flexDirection: 'row', alignItems: 'center', justifyContent: 'center', gap: 10, paddingHorizontal: 24, borderRadius: 14, backgroundColor: '#8b1a2e', boxShadow: '0px 6px 16px rgba(139,26,46,0.25)' },
  startText: { flexShrink: 1, textAlign: 'center', fontFamily: 'Inter_600SemiBold', color: '#fff', includeFontPadding: false },
  login: { padding: 8, minHeight: layout.touchTarget, minWidth: layout.touchTarget, justifyContent: 'center' },
  loginText: { fontFamily: 'Inter_400Regular', fontSize: 13, lineHeight: 16, color: '#fff', textAlign: 'center' },
  loginAccent: { fontFamily: 'Inter_500Medium', color: '#f5a915' },
});
