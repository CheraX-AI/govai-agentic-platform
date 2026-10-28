import { useEffect, useRef, useState } from 'react';
import { StatusBar } from 'expo-status-bar';
import { Image, Keyboard, Pressable, ScrollView, StyleSheet, Text, TextInput, View, useWindowDimensions } from 'react-native';
import { Screen } from './components/Screen';
import { layout, typography } from './layout';

export const loginAssets = {
  topRibbon: require('./assets/figma/login/top-ribbon.png'),
  skyline: require('./assets/figma/login/skyline.png'),
  bottomRibbon: require('./assets/figma/login/bottom-ribbon.png'),
  logo: require('./assets/figma/login/logo.png'),
  user: require('./assets/figma/login/user.png'),
  lock: require('./assets/figma/login/lock.png'),
  eye: require('./assets/figma/login/eye.png'),
  fingerprint: require('./assets/figma/login/fingerprint.png'),
};

type Viewport = { width: number; height: number };
const sameViewport = (a: Viewport, b: Viewport) => Math.abs(a.width - b.width) < 0.5 && Math.abs(a.height - b.height) < 0.5;

export function LoginScreen({ onBack, onSignUp, onForgotPassword, onDemoValidated }: { onBack: () => void; onSignUp: () => void; onForgotPassword: () => void; onDemoValidated?: () => void }) {
  const transitioning = useRef(false);
  const demo = __DEV__ && process.env.EXPO_PUBLIC_CONNECTED_UI_DEMO === '1' && !!onDemoValidated;
  const initialNotice = demo ? 'UI demo only. Credentials are not verified; valid fields open Chat Home.' : 'Sign-in is not connected yet.';
  // Credentials stay only in this mounted form, never in app navigation/state storage.
  const [identifier, setIdentifier] = useState('');
  const [password, setPassword] = useState('');
  const [visible, setVisible] = useState(false);
  const [submitted, setSubmitted] = useState(false);
  const [notice, setNotice] = useState(initialNotice);
  const [keyboardOverlap, setKeyboardOverlap] = useState(0);
  const [closedViewport, setClosedViewport] = useState({ width: 0, height: 0 });
  const [formHeight, setFormHeight] = useState(0);
  const [headerHeight, setHeaderHeight] = useState(48);
  const root = useRef<View>(null);
  const scrollViewport = useRef<View>(null);
  const scroll = useRef<ScrollView>(null);
  const identifierInput = useRef<TextInput>(null);
  const passwordInput = useRef<TextInput>(null);
  const keyboardTop = useRef<number | null>(null);
  const identifierGroup = useRef<View>(null);
  const passwordGroup = useRef<View>(null);
  const focusedField = useRef<'identifier' | 'password' | null>(null);
  const scrollOffset = useRef(0);
  const adjustmentFrame = useRef<number | null>(null);
  const [artworkViewport, setArtworkViewport] = useState({ width: 0, height: 0 });
  const latestViewport = useRef({ artwork: { width: 0, height: 0 }, content: { width: 0, height: 0 } });
  const baselineViewport = useRef({ artwork: { width: 0, height: 0 }, content: { width: 0, height: 0 } });
  const keyboardPhase = useRef<'idle' | 'opening' | 'open' | 'closing'>('idle');
  const viewportFrame = useRef<number | null>(null);
  const transitionTimeout = useRef<ReturnType<typeof setTimeout> | null>(null);
  const { width } = useWindowDimensions();
  const artworkScale = Math.min(width / 400, 1.25);
  // Decoration uses the settled keyboard-free baseline. Field visibility always
  // uses live native measurements, including when a real resize occurs with IME open.
  const artworkHeight = artworkViewport.height;
  const logoHeight = Math.min(198, Math.max(112, closedViewport.height - headerHeight - formHeight - 35));
  const logoWidth = logoHeight * 196 / 198;
  const identifierError = submitted && !identifier.trim();
  const passwordError = submitted && !password.trim();

  function clearTransitionTimeout() {
    if (transitionTimeout.current !== null) clearTimeout(transitionTimeout.current);
    transitionTimeout.current = null;
  }
  function commitViewportMeasurements() {
    const actual = latestViewport.current;
    const baseline = baselineViewport.current;
    const widthChanged = (baseline.artwork.width > 0 && Math.abs(actual.artwork.width - baseline.artwork.width) >= 0.5)
      || (baseline.content.width > 0 && Math.abs(actual.content.width - baseline.content.width) >= 0.5);
    const keyboardVisible = Keyboard.isVisible() || keyboardTop.current !== null;
    // Android can deliver resized onLayout events before keyboardDidShow.
    // Focus arms an opening guard; keyboard events settle the phase.
    if (!widthChanged && (keyboardVisible || keyboardPhase.current === 'opening')) return;
    if (keyboardPhase.current === 'closing') {
      const restored = actual.artwork.height >= baseline.artwork.height - 0.5
        && actual.content.height >= baseline.content.height - 0.5;
      if (!widthChanged && !restored) return;
      keyboardPhase.current = 'idle';
      clearTransitionTimeout();
    }
    // Accept both increases and decreases. Never feed content/logo size into
    // these measurements: both observed viewports are bounded native containers.
    if (actual.artwork.width > 0 && !sameViewport(actual.artwork, baseline.artwork)) {
      baselineViewport.current.artwork = actual.artwork;
      setArtworkViewport(actual.artwork);
    }
    if (actual.content.width > 0 && !sameViewport(actual.content, baseline.content)) {
      baselineViewport.current.content = actual.content;
      setClosedViewport(actual.content);
    }
  }
  function scheduleViewportCheck() {
    if (viewportFrame.current !== null) cancelAnimationFrame(viewportFrame.current);
    viewportFrame.current = requestAnimationFrame(() => {
      viewportFrame.current = requestAnimationFrame(() => {
        viewportFrame.current = null;
        commitViewportMeasurements();
      });
    });
  }

  function adjustVisibility() {
    const top = keyboardTop.current;
    if (top === null) return;
    // One window-coordinate strategy: resized viewports produce no extra
    // keyboard padding; overlays add only the uncovered portion.
    scrollViewport.current?.measureInWindow((_x, y, _width, viewportHeight) => {
      if (keyboardTop.current !== top) return;
      const bottom = Math.min(y + viewportHeight, top);
      const overlap = Math.max(0, y + viewportHeight - top);
      setKeyboardOverlap(previous => Math.abs(previous - overlap) < 0.5 ? previous : overlap);
      const group = focusedField.current === 'identifier' ? identifierGroup.current
        : focusedField.current === 'password' ? passwordGroup.current : null;
      group?.measureInWindow((_fieldX, fieldY, _fieldWidth, fieldHeight) => {
        if (keyboardTop.current !== top) return;
        // Reveal the field and its error, rather than aligning it to the top.
        const delta = fieldHeight > bottom - y - 24 ? fieldY - (y + 12)
          : fieldY < y + 12 ? fieldY - (y + 12)
          : fieldY + fieldHeight > bottom - 12 ? fieldY + fieldHeight - (bottom - 12) : 0;
        if (Math.abs(delta) > 1) scroll.current?.scrollTo({ y: Math.max(0, scrollOffset.current + delta), animated: false });
      });
    });
  }
  function scheduleVisibilityCheck() {
    if (adjustmentFrame.current !== null) cancelAnimationFrame(adjustmentFrame.current);
    // Coalesce focus, keyboard and layout events; allow native resizing and
    // error layout to commit before taking a single set of measurements.
    adjustmentFrame.current = requestAnimationFrame(() => {
      adjustmentFrame.current = requestAnimationFrame(() => {
        adjustmentFrame.current = null;
        adjustVisibility();
      });
    });
  }
  useEffect(() => {
    const show = Keyboard.addListener('keyboardDidShow', event => {
      clearTransitionTimeout();
      keyboardPhase.current = 'open';
      keyboardTop.current = event.endCoordinates.screenY;
      scheduleViewportCheck();
      scheduleVisibilityCheck();
    });
    const hide = Keyboard.addListener('keyboardDidHide', () => {
      keyboardTop.current = null;
      keyboardPhase.current = 'closing';
      setKeyboardOverlap(0);
      if (adjustmentFrame.current !== null) cancelAnimationFrame(adjustmentFrame.current);
      adjustmentFrame.current = null;
      clearTransitionTimeout();
      // A real same-width resize during editing can leave a smaller window.
      // Give IME restoration time to settle, then accept that actual size.
      // This timer is armed by the keyboard event, never by measurement updates.
      transitionTimeout.current = setTimeout(() => {
        transitionTimeout.current = null;
        keyboardPhase.current = 'idle';
        scheduleViewportCheck();
      }, 250);
      scheduleViewportCheck();
    });
    const changeFrame = Keyboard.addListener('keyboardDidChangeFrame', event => {
      if (!Keyboard.isVisible() && keyboardTop.current === null) return;
      keyboardTop.current = event.endCoordinates.screenY;
      scheduleVisibilityCheck();
    });
    const identifierField = identifierInput.current;
    const passwordField = passwordInput.current;
    return () => {
      transitioning.current = true;
      show.remove(); hide.remove(); changeFrame.remove();
      clearTransitionTimeout();
      if (viewportFrame.current !== null) cancelAnimationFrame(viewportFrame.current);
      if (adjustmentFrame.current !== null) cancelAnimationFrame(adjustmentFrame.current);
      identifierField?.clear(); passwordField?.clear();
      Keyboard.dismiss();
    };
  }, []);
  useEffect(() => { scheduleVisibilityCheck(); }, [identifierError, passwordError]);

  function leave() {
    transitioning.current = true;
    identifierInput.current?.clear(); passwordInput.current?.clear();
    setIdentifier(''); setPassword(''); setVisible(false);
    Keyboard.dismiss(); onBack();
  }
  function openForgotPassword() {
    transitioning.current = true;
    identifierInput.current?.clear(); passwordInput.current?.clear();
    setIdentifier(''); setPassword(''); setVisible(false); setSubmitted(false);
    setNotice(initialNotice);
    Keyboard.dismiss(); onForgotPassword();
  }
  function focusField(field: 'identifier' | 'password') {
    focusedField.current = field;
    const metrics = Keyboard.metrics();
    if (metrics) {
      clearTransitionTimeout();
      keyboardPhase.current = 'open';
      keyboardTop.current = metrics.screenY;
    }
    if (!Keyboard.isVisible() && keyboardTop.current === null) {
      keyboardPhase.current = 'opening';
      clearTransitionTimeout();
      // Hardware-keyboard focus may never produce an IME event. Bound that
      // pending phase so a focused field cannot freeze genuine window resizing.
      transitionTimeout.current = setTimeout(() => {
        transitionTimeout.current = null;
        if (keyboardPhase.current === 'opening' && !Keyboard.isVisible()) {
          keyboardPhase.current = 'idle';
          scheduleViewportCheck();
        }
      }, 500);
    }
    scheduleVisibilityCheck();
  }
  function blurField(field: 'identifier' | 'password') {
    if (focusedField.current === field) focusedField.current = null;
    if (keyboardPhase.current === 'opening' && !Keyboard.isVisible()) {
      clearTransitionTimeout();
      keyboardPhase.current = 'idle';
      scheduleViewportCheck();
    }
  }
  function submit() {
    if (transitioning.current) return;
    setSubmitted(true);
    if (!identifier.trim()) { identifierInput.current?.focus(); return; }
    if (!password.trim()) { passwordInput.current?.focus(); return; }
    if (__DEV__ && process.env.EXPO_PUBLIC_CONNECTED_UI_DEMO === '1' && onDemoValidated) {
      transitioning.current = true;
      identifierInput.current?.clear(); passwordInput.current?.clear();
      setIdentifier(''); setPassword(''); setVisible(false); setSubmitted(false); setNotice(initialNotice);
      focusedField.current = null; Keyboard.dismiss(); onDemoValidated(); return;
    }
    Keyboard.dismiss();
    setNotice('Sign-in is not connected yet. No sign-in request was sent.');
  }

  return (
    <View ref={root} onLayout={({ nativeEvent: { layout: measured } }) => {
      latestViewport.current.artwork = { width: measured.width, height: measured.height };
      scheduleViewportCheck();
      scheduleVisibilityCheck();
    }} style={styles.root}>
      <StatusBar style="dark" />
      <View pointerEvents="none" accessible={false} importantForAccessibility="no-hide-descendants" style={StyleSheet.absoluteFill}>
        <Image source={loginAssets.topRibbon} resizeMode="cover" style={[styles.topRibbon, { width: 272 * artworkScale, height: 296 * artworkScale }]} />
        <Image source={loginAssets.skyline} resizeMode="contain" style={[styles.skyline, { width: 723 * artworkScale, height: 451 * artworkScale, top: artworkHeight - 445 * artworkScale }]} />
        <Image source={loginAssets.bottomRibbon} resizeMode="cover" style={[styles.bottomRibbon, { width: 237 * artworkScale, height: 227 * artworkScale, top: artworkHeight - 227 * artworkScale + 8 }]} />
      </View>
      <Screen scroll={false} contentStyle={styles.screenContent}>
        <View ref={scrollViewport} style={styles.screenContent} onLayout={({ nativeEvent: { layout: measured } }) => {
          latestViewport.current.content = { width: measured.width, height: measured.height };
          scheduleViewportCheck();
          scheduleVisibilityCheck();
        }}>
        <ScrollView ref={scroll} showsVerticalScrollIndicator={false} keyboardShouldPersistTaps="handled" keyboardDismissMode="on-drag"
          automaticallyAdjustKeyboardInsets={false} automaticallyAdjustContentInsets={false} contentInsetAdjustmentBehavior="never"
          onScroll={event => { scrollOffset.current = event.nativeEvent.contentOffset.y; }} scrollEventThrottle={16}
          onContentSizeChange={scheduleVisibilityCheck}
          contentContainerStyle={[styles.scrollContent, { paddingBottom: keyboardOverlap + 8 }]}>
          <View style={styles.header} onLayout={event => setHeaderHeight(event.nativeEvent.layout.height)}>
          <Pressable onPress={leave} accessibilityRole="button" accessibilityLabel={"Back to previous screen"} style={styles.back}>
            <Text style={styles.backText}>{"Back"}</Text>
          </Pressable>
          <View style={styles.vision}><Text style={styles.visionText}>{"FOR A\nSMARTER\nSRI LANKA"}</Text><View style={styles.accent} /></View>
          </View>
          <View accessible accessibilityRole="image" accessibilityLabel={"GovAI. People, services, a smarter Sri Lanka."} style={[styles.logoSlot, { width: logoWidth, height: logoHeight }]}>
            <Image source={loginAssets.logo} accessible={false} resizeMode="contain" style={[styles.logo, {
              width: logoWidth * 1.5417, height: logoHeight * 2.7132, left: -logoWidth * 0.3013, top: -logoHeight * 0.636,
            }]} />
          </View>
          <View style={styles.form} onLayout={event => setFormHeight(event.nativeEvent.layout.height)}>
            <View style={styles.formHeading}>
            <Text accessibilityRole="header" style={[typography.heading, styles.heading]}>{"Welcome Back"}</Text>
            <Text style={[typography.body, styles.subtitle]}>{"Log in to continue to GovAI"}</Text>
            </View>
            <Text accessibilityLiveRegion="polite" style={styles.notice}>{notice}</Text>
            <View ref={identifierGroup} onLayout={scheduleVisibilityCheck}>
              <View style={[styles.field, identifierError && styles.invalid]}>
                <Image source={loginAssets.user} accessible={false} style={styles.fieldIcon} />
                <TextInput ref={identifierInput} value={identifier} onChangeText={setIdentifier} placeholder={"Email or National ID"}
                  placeholderTextColor="#9ca3af" accessibilityLabel={identifierError ? "Email or National ID. Required." : "Email or National ID"}
                  autoCapitalize="none" autoCorrect={false} spellCheck={false} autoComplete="off" importantForAutofill="no"
                  returnKeyType="next" submitBehavior="submit" onSubmitEditing={() => passwordInput.current?.focus()}
                  onFocus={() => focusField('identifier')} onBlur={() => blurField('identifier')} style={styles.input} />
              </View>
              {identifierError && <Text accessibilityLiveRegion="polite" style={styles.error}>{"Enter your email or National ID."}</Text>}
            </View>
            <View ref={passwordGroup} onLayout={scheduleVisibilityCheck}>
              <View style={[styles.field, passwordError && styles.invalid]}>
                <Image source={loginAssets.lock} accessible={false} style={styles.fieldIcon} />
                <TextInput ref={passwordInput} value={password} onChangeText={setPassword} placeholder={"Password"}
                  placeholderTextColor="#9ca3af" accessibilityLabel={passwordError ? "Password. Required." : "Password"}
                  secureTextEntry={!visible} autoCapitalize="none" autoCorrect={false} spellCheck={false}
                  autoComplete="off" importantForAutofill="no" returnKeyType="done" onSubmitEditing={submit}
                  onFocus={() => focusField('password')} onBlur={() => blurField('password')} style={styles.input} />
                <Pressable onPress={() => setVisible(previous => !previous)} accessibilityRole="button"
                  accessibilityLabel={visible ? "Hide password" : "Show password"} accessibilityState={{ selected: visible }} style={styles.eye}>
                  <Image source={loginAssets.eye} accessible={false} style={styles.fieldIcon} />
                </Pressable>
              </View>
              {passwordError && <Text accessibilityLiveRegion="polite" style={styles.error}>{"Enter your password."}</Text>}
            </View>
            <Pressable onPress={openForgotPassword} accessibilityRole="link" accessibilityLabel="Forgot Password?" style={styles.forgot}>
              <Text style={styles.link}>{"Forgot Password?"}</Text>
            </Pressable>
            <Pressable onPress={submit} accessibilityRole="button" accessibilityLabel={demo ? "Log In. Open Chat Home in UI demo only; credentials are not verified." : "Log In. Sign-in is not connected; checks required fields only."} style={styles.primary}>
              <Text style={[typography.action, styles.primaryText]}>{"Log In →"}</Text>
            </Pressable>
            <View style={styles.divider}><View style={styles.rule} /><Text style={styles.or}>{"OR"}</Text><View style={styles.rule} /></View>
            <Pressable disabled accessibilityRole="button" accessibilityState={{ disabled: true }} accessibilityLabel={"Log in with Digital ID. Unavailable."} style={styles.digital}>
              <Image source={loginAssets.fingerprint} accessible={false} style={styles.fingerprint} />
              <Text style={styles.digitalText}>{"Log in with Digital ID →"}</Text>
            </Pressable>
            <View style={styles.signup}>
              <Text style={styles.subtitle}>{"Don’t have an account?"}</Text>
              <Pressable onPress={() => { transitioning.current = true; identifierInput.current?.clear(); passwordInput.current?.clear(); setIdentifier(''); setPassword(''); setVisible(false); Keyboard.dismiss(); onSignUp(); }} accessibilityRole="link" accessibilityLabel="Sign up" style={styles.signupLink}>
                <Text style={styles.link}>{"Sign up"}</Text>
              </Pressable>
            </View>
          </View>
          <Text style={styles.country}>{"SRI LANKA"}</Text>
        </ScrollView>
        </View>
      </Screen>
    </View>
  );
}

const styles = StyleSheet.create({
  root: { flex: 1, backgroundColor: '#fff' },
  topRibbon: { position: 'absolute', left: -9, top: -14 },
  skyline: { position: 'absolute', left: -164, opacity: 0.45 },
  bottomRibbon: { position: 'absolute', right: -7 },
  scrollContent: { flexGrow: 1 },
  screenContent: { flex: 1 },
  header: { flexDirection: 'row', alignItems: 'center', justifyContent: 'space-between', gap: 12 },
  back: { alignSelf: 'flex-start', minWidth: layout.touchTarget, minHeight: layout.touchTarget, justifyContent: 'center', paddingHorizontal: 12, backgroundColor: '#ffffffed', borderRadius: 12 },
  backText: { fontFamily: 'Inter_600SemiBold', color: '#8b1a2e', fontSize: 14 },
  vision: { flexShrink: 1, alignItems: 'flex-end', gap: 6 },
  visionText: { fontFamily: 'Inter_500Medium', color: '#666', fontSize: 9, lineHeight: 13, letterSpacing: 2, textAlign: 'right' },
  accent: { height: 2, width: 36, backgroundColor: '#f5a915' },
  logoSlot: { alignSelf: 'center', overflow: 'hidden', marginBottom: 8 },
  logo: { position: 'absolute' },
  form: { gap: 8, width: '100%' },
  formHeading: { gap: 6 },
  heading: { fontFamily: 'Inter_700Bold', color: '#142033', textAlign: 'center' },
  subtitle: { fontFamily: 'Inter_400Regular', fontSize: 14, lineHeight: 20, color: '#666', textAlign: 'center' },
  notice: { fontFamily: 'Inter_400Regular', fontSize: 13, lineHeight: 18, color: '#8b1a2e', backgroundColor: '#fff' },
  field: { flexDirection: 'row', alignItems: 'center', gap: 10, paddingLeft: 16, paddingRight: 8, minHeight: 52, backgroundColor: '#fff', borderWidth: 1, borderColor: '#e5e7eb', borderRadius: 12 },
  fieldIcon: { width: 18, height: 18 },
  input: { flex: 1, minWidth: 0, fontFamily: 'Inter_400Regular', fontSize: 14, color: '#142033', paddingVertical: 14 },
  eye: { minWidth: layout.touchTarget, minHeight: layout.touchTarget, alignItems: 'center', justifyContent: 'center' },
  invalid: { borderColor: '#a30f2d' },
  error: { color: '#a30f2d', fontFamily: 'Inter_400Regular', fontSize: 13, lineHeight: 18, paddingTop: 4 },
  forgot: { minHeight: layout.touchTarget, minWidth: layout.touchTarget, alignSelf: 'flex-end', justifyContent: 'center' },
  link: { fontFamily: 'Inter_600SemiBold', fontSize: 13, lineHeight: 18, color: '#a30f2d' },
  primary: { minHeight: 52, borderRadius: 14, padding: 16, backgroundColor: '#8b1a2e', alignItems: 'center', justifyContent: 'center', boxShadow: '0px 6px 8px rgba(139,26,46,0.25)' },
  primaryText: { fontFamily: 'Inter_600SemiBold', color: '#fff', textAlign: 'center' },
  divider: { flexDirection: 'row', alignItems: 'center', gap: 12, paddingVertical: 4 },
  rule: { flex: 1, height: 1, backgroundColor: '#e5e7eb' },
  or: { fontFamily: 'Inter_500Medium', fontSize: 12, color: '#9ca3af' },
  digital: { minHeight: 52, padding: 14, flexDirection: 'row', alignItems: 'center', justifyContent: 'center', gap: 10, borderWidth: 1, borderColor: '#e5e7eb', borderRadius: 14, backgroundColor: '#fff' },
  fingerprint: { width: 22, height: 22 },
  digitalText: { flexShrink: 1, fontFamily: 'Inter_500Medium', fontSize: 14, lineHeight: 20, color: '#444', textAlign: 'center' },
  signup: { flexDirection: 'row', flexWrap: 'wrap', alignItems: 'center', justifyContent: 'center', gap: 4 },
  signupLink: { minWidth: layout.touchTarget, minHeight: layout.touchTarget, justifyContent: 'center' },
  country: { paddingTop: 8, fontFamily: 'Inter_400Regular', color: '#9ca3af', fontSize: 9, letterSpacing: 4, textAlign: 'center' },
});
