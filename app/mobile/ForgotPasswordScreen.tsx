import { useEffect, useRef, useState } from 'react';
import { StatusBar } from 'expo-status-bar';
import { Image, Keyboard, Pressable, ScrollView, StyleSheet, Text, TextInput, View, useWindowDimensions } from 'react-native';
import { Screen } from './components/Screen';
import { layout, typography } from './layout';
import { loginAssets } from './LoginScreen';
import { registerAssets } from './RegisterScreen';

export const forgotPasswordAssets = {
  lock: require('./assets/figma/forgot-password/lock.png'),
  identity: require('./assets/figma/forgot-password/identity.png'),
  info: require('./assets/figma/forgot-password/info.png'),
};
const description = 'Enter your NIC number. Password recovery is not connected in this UI-only preview.';
const disconnectedNotice = 'Password recovery is not connected. No request was sent and no password was changed.';
type Viewport = { width: number; height: number };

export function ForgotPasswordScreen({ onLogin, onDemoValidated }: { onLogin: () => void; onDemoValidated?: () => void }) {
  const transitioning = useRef(false);
  const [nic, setNic] = useState('');
  const [submitted, setSubmitted] = useState(false);
  const [notice, setNotice] = useState('');
  const [focused, setFocused] = useState(false);
  const [closedViewport, setClosedViewport] = useState<Viewport>({ width: 0, height: 0 });
  const [cardHeight, setCardHeight] = useState(0);
  const [visionHeight, setVisionHeight] = useState(47);
  const [keyboardOverlap, setKeyboardOverlap] = useState(0);
  const viewport = useRef<View>(null);
  const fieldGroup = useRef<View>(null);
  const input = useRef<TextInput>(null);
  const scroll = useRef<ScrollView>(null);
  const liveViewport = useRef<Viewport>({ width: 0, height: 0 });
  const baseline = useRef<Viewport>({ width: 0, height: 0 });
  const keyboardTop = useRef<number | null>(null);
  const phase = useRef<'idle' | 'opening' | 'open' | 'closing'>('idle');
  const fieldFocused = useRef(false);
  const offset = useRef(0);
  const layoutFrame = useRef<number | null>(null);
  const visibilityFrame = useRef<number | null>(null);
  const transitionTimer = useRef<ReturnType<typeof setTimeout> | null>(null);
  const { width, fontScale } = useWindowDimensions();
  const error = submitted && !nic.trim();
  const contentWidth = closedViewport.width || Math.min(layout.contentWidth, Math.max(0, width - 32));
  const maximumLogoWidth = Math.min(150, Math.max(0, contentWidth - 2 * (86 + 9 + 4)));
  const maximumLogoHeight = maximumLogoWidth * 151.531 / 150;
  const intrinsicCardHeight = cardHeight || 450 * fontScale;
  // Only intrinsic text/card measurements affect the header. The card itself has
  // no flexible height, so header changes cannot feed back into its measurement.
  const headerGap = Math.max(12, Math.min(32.469, closedViewport.height - intrinsicCardHeight - maximumLogoHeight - 42));
  const logoHeight = Math.min(maximumLogoHeight, Math.max(Math.min(64, maximumLogoHeight),
    closedViewport.height - intrinsicCardHeight - headerGap - 42));
  const logoWidth = logoHeight * 150 / 151.531;
  const visionTop = Math.max(6, Math.min(32, 6 + (logoHeight - visionHeight) / 2));
  const headerHeight = Math.max(6 + logoHeight, visionTop + visionHeight, layout.touchTarget + 5);
  const artworkScale = Math.min(width / 400, 1.25);

  function clearTimer() {
    if (transitionTimer.current !== null) clearTimeout(transitionTimer.current);
    transitionTimer.current = null;
  }
  function scheduleLayout() {
    if (layoutFrame.current !== null) cancelAnimationFrame(layoutFrame.current);
    layoutFrame.current = requestAnimationFrame(() => {
      layoutFrame.current = requestAnimationFrame(() => {
        layoutFrame.current = null;
        const current = liveViewport.current, previous = baseline.current;
        const widthChanged = previous.width > 0 && Math.abs(current.width - previous.width) >= 0.5;
        if (!widthChanged && (Keyboard.isVisible() || keyboardTop.current !== null || phase.current === 'opening')) return;
        if (phase.current === 'closing') {
          if (!widthChanged && current.height < previous.height - 0.5) return;
          phase.current = 'idle'; clearTimer();
        }
        if (current.width > 0 && (Math.abs(current.width - previous.width) >= 0.5 || Math.abs(current.height - previous.height) >= 0.5)) {
          baseline.current = current; setClosedViewport(current);
        }
      });
    });
  }
  function adjustVisibility() {
    const top = keyboardTop.current;
    if (top === null) return;
    viewport.current?.measureInWindow((_x, y, _w, height) => {
      if (keyboardTop.current !== top) return;
      const bottom = Math.min(y + height, top);
      const overlap = Math.max(0, y + height - top);
      setKeyboardOverlap(previous => Math.abs(previous - overlap) < 0.5 ? previous : overlap);
      if (!fieldFocused.current) return;
      fieldGroup.current?.measureInWindow((_fieldX, fieldY, _fieldWidth, fieldHeight) => {
        if (keyboardTop.current !== top || !fieldFocused.current) return;
        const delta = fieldHeight > bottom - y - 24 ? fieldY - (y + 12)
          : fieldY < y + 12 ? fieldY - (y + 12)
          : fieldY + fieldHeight > bottom - 12 ? fieldY + fieldHeight - (bottom - 12) : 0;
        if (Math.abs(delta) > 1) scroll.current?.scrollTo({ y: Math.max(0, offset.current + delta), animated: false });
      });
    });
  }
  function scheduleVisibility() {
    if (visibilityFrame.current !== null) cancelAnimationFrame(visibilityFrame.current);
    visibilityFrame.current = requestAnimationFrame(() => {
      visibilityFrame.current = requestAnimationFrame(() => { visibilityFrame.current = null; adjustVisibility(); });
    });
  }
  useEffect(() => {
    const show = Keyboard.addListener('keyboardDidShow', event => {
      clearTimer(); phase.current = 'open'; keyboardTop.current = event.endCoordinates.screenY;
      scheduleLayout(); scheduleVisibility();
    });
    const hide = Keyboard.addListener('keyboardDidHide', () => {
      keyboardTop.current = null; phase.current = 'closing'; setKeyboardOverlap(0);
      if (visibilityFrame.current !== null) cancelAnimationFrame(visibilityFrame.current);
      visibilityFrame.current = null; clearTimer();
      transitionTimer.current = setTimeout(() => { transitionTimer.current = null; phase.current = 'idle'; scheduleLayout(); }, 250);
      scheduleLayout();
    });
    const frame = Keyboard.addListener('keyboardDidChangeFrame', event => {
      if (!Keyboard.isVisible() && keyboardTop.current === null) return;
      keyboardTop.current = event.endCoordinates.screenY; scheduleVisibility();
    });
    const mountedInput = input.current;
    return () => {
      show.remove(); hide.remove(); frame.remove(); clearTimer();
      if (layoutFrame.current !== null) cancelAnimationFrame(layoutFrame.current);
      if (visibilityFrame.current !== null) cancelAnimationFrame(visibilityFrame.current);
      mountedInput?.clear(); Keyboard.dismiss();
    };
  }, []);
  useEffect(() => { scheduleVisibility(); }, [error]);
  function focus() {
    fieldFocused.current = true; setFocused(true);
    const metrics = Keyboard.metrics();
    if (metrics) { clearTimer(); phase.current = 'open'; keyboardTop.current = metrics.screenY; }
    else if (!Keyboard.isVisible() && keyboardTop.current === null) {
      phase.current = 'opening'; clearTimer();
      transitionTimer.current = setTimeout(() => {
        transitionTimer.current = null;
        if (phase.current === 'opening' && !Keyboard.isVisible()) { phase.current = 'idle'; scheduleLayout(); }
      }, 500);
    }
    scheduleVisibility();
  }
  function blur() {
    fieldFocused.current = false; setFocused(false);
    if (phase.current === 'opening' && !Keyboard.isVisible()) { clearTimer(); phase.current = 'idle'; scheduleLayout(); }
  }
  function leave() {
    input.current?.clear(); setNic(''); setSubmitted(false); setNotice(''); setFocused(false); fieldFocused.current = false;
    Keyboard.dismiss(); onLogin();
  }
  function submit() {
    if (transitioning.current) return;
    setSubmitted(true);
    if (!nic.trim()) { setNotice(''); input.current?.focus(); return; }
    if (__DEV__ && process.env.EXPO_PUBLIC_CONNECTED_UI_DEMO === '1' && onDemoValidated) {
      transitioning.current = true;
      input.current?.clear(); setNic(''); setSubmitted(false); setNotice('');
      setFocused(false); fieldFocused.current = false;
      Keyboard.dismiss(); onDemoValidated(); return;
    }
    Keyboard.dismiss(); setNotice(disconnectedNotice);
  }
  return <View style={styles.root}>
    <StatusBar style="dark" />
    <View pointerEvents="none" accessible={false} importantForAccessibility="no-hide-descendants" style={StyleSheet.absoluteFill}>
      <Image source={loginAssets.topRibbon} accessible={false} resizeMode="cover" style={[styles.topRibbon, { width: 272 * artworkScale, height: 296 * artworkScale }]} />
      <Image source={loginAssets.skyline} accessible={false} resizeMode="contain" style={[styles.skyline, { width: 723 * artworkScale, height: 451 * artworkScale }]} />
      <Image source={loginAssets.bottomRibbon} accessible={false} resizeMode="cover" style={[styles.bottomRibbon, { width: 237 * artworkScale, height: 227 * artworkScale }]} />
    </View>
    <Screen scroll={false} horizontalPadding={16} contentStyle={styles.screen}>
      <View ref={viewport} style={styles.screen} onLayout={({ nativeEvent: { layout: measured } }) => {
        liveViewport.current = { width: measured.width, height: measured.height }; scheduleLayout(); scheduleVisibility();
      }}>
        <ScrollView ref={scroll} showsVerticalScrollIndicator={false} alwaysBounceVertical={false} overScrollMode="never"
          keyboardShouldPersistTaps="handled" keyboardDismissMode="on-drag" automaticallyAdjustKeyboardInsets={false}
          automaticallyAdjustContentInsets={false} contentInsetAdjustmentBehavior="never"
          onScroll={event => { offset.current = event.nativeEvent.contentOffset.y; }} scrollEventThrottle={16}
          onContentSizeChange={scheduleVisibility} contentContainerStyle={[styles.content, { paddingBottom: keyboardOverlap + 8 }]}>
          <View style={[styles.header, { height: headerHeight }]}>
            <Pressable accessibilityRole="button" accessibilityLabel="Back to Login" onPress={leave} style={styles.back}>
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
            const measured = event.nativeEvent.layout.height; setCardHeight(previous => Math.abs(previous - measured) < 0.5 ? previous : measured);
          }}>
            <View style={styles.badge}><Image source={forgotPasswordAssets.lock} accessible={false} style={styles.lockIcon} /></View>
            <View style={styles.cardHeader}>
              <Text accessibilityRole="header" style={styles.heading}>Forgot your password?</Text>
              <Text style={styles.description}>{description}</Text>
            </View>
            <View ref={fieldGroup} onLayout={scheduleVisibility} style={styles.fieldGroup}>
              <Text style={styles.label}>National ID number</Text>
              <View style={[styles.field, (focused || error) && styles.emphasized]}>
                <Image source={forgotPasswordAssets.identity} accessible={false} style={styles.identityIcon} />
                <TextInput ref={input} value={nic} onChangeText={setNic} placeholder="Enter your National ID number" placeholderTextColor="#666666"
                  accessibilityLabel={`National ID number. Required.${error ? ' Enter your National ID number.' : ''}`}
                  autoCapitalize="none" autoCorrect={false} spellCheck={false} autoComplete="off" importantForAutofill="no"
                  returnKeyType="done" onSubmitEditing={submit} onFocus={focus} onBlur={blur} style={styles.input} />
              </View>
              {error && <Text accessibilityLiveRegion="polite" style={styles.error}>Enter your National ID number.</Text>}
            </View>
            <View style={styles.help}><Image source={forgotPasswordAssets.info} accessible={false} style={styles.infoIcon} />
              <Text style={styles.helpText}>Recovery support details are not available in this UI-only preview.</Text></View>
            {!!notice && <Text accessibilityLiveRegion="polite" style={styles.notice}>{notice}</Text>}
            <Pressable onPress={submit} accessibilityRole="button" accessibilityLabel="Send reset link. UI-only required-field check; password recovery is not connected." style={styles.primary}>
              <Text style={[typography.action, styles.primaryText]}>Send reset link</Text>
            </Pressable>
            <View style={styles.loginRow}><Text style={styles.description}>Remembered it?</Text>
              <Pressable onPress={leave} accessibilityRole="link" accessibilityLabel="Log in" style={styles.loginLink}><Text style={styles.link}>Log in</Text></Pressable></View>
          </View>
          <Text style={styles.country}>SRI LANKA</Text>
        </ScrollView>
      </View>
    </Screen>
  </View>;
}
const styles = StyleSheet.create({
  root: { flex: 1, backgroundColor: '#fff' }, screen: { flex: 1 }, content: { flexGrow: 1 },
  topRibbon: { position: 'absolute', left: -9, top: -14 }, skyline: { position: 'absolute', left: -164, bottom: -6, opacity: 0.45 },
  bottomRibbon: { position: 'absolute', right: -7, bottom: -8 },
  header: { alignItems: 'center', width: '100%', flexShrink: 0 },
  back: { position: 'absolute', left: -5, top: 5, width: layout.touchTarget, height: layout.touchTarget, alignItems: 'center', justifyContent: 'center', zIndex: 1 },
  backCircle: { width: 38, height: 38, borderRadius: 9999, backgroundColor: '#fff', alignItems: 'center', justifyContent: 'center', boxShadow: '0px 1px 4px rgba(0,0,0,0.04)' },
  backIcon: { width: 20, height: 20 }, logoSlot: { marginTop: 6, overflow: 'hidden' },
  vision: { position: 'absolute', right: 9, width: 86, alignItems: 'flex-end', gap: 6 },
  visionText: { fontFamily: 'Inter_500Medium', color: '#666', fontSize: 9, lineHeight: 13, letterSpacing: 2, textAlign: 'right' },
  accent: { width: 36, height: 2, backgroundColor: '#f5a915' },
  card: { width: '100%', flexShrink: 0, paddingHorizontal: 20, paddingVertical: 24, gap: 16, borderRadius: 24, borderWidth: 1, borderColor: '#f1f5f9', backgroundColor: '#fff', boxShadow: '0px 10px 22px rgba(0,0,0,0.1)' },
  badge: { width: 72, height: 72, borderRadius: 9999, backgroundColor: '#fcf3f5', alignItems: 'center', justifyContent: 'center' },
  lockIcon: { width: 34, height: 34 }, identityIcon: { width: 18, height: 18 }, infoIcon: { width: 16, height: 16 },
  cardHeader: { gap: 6 }, heading: { fontFamily: 'Inter_700Bold', fontSize: 20, lineHeight: 28, color: '#142033' },
  description: { fontFamily: 'Inter_400Regular', fontSize: 13, lineHeight: 18, color: '#666' },
  fieldGroup: { gap: 6 }, label: { fontFamily: 'Inter_500Medium', fontSize: 13, lineHeight: 18, color: '#142033' },
  field: { minHeight: layout.touchTarget, flexDirection: 'row', alignItems: 'center', gap: 10, paddingHorizontal: 14, borderRadius: 12, borderWidth: 1.5, borderColor: '#e5e7eb', backgroundColor: '#fff', boxShadow: '0px 1px 4px rgba(0,0,0,0.04)' },
  emphasized: { borderColor: '#a30f2d' }, input: { flex: 1, minWidth: 0, fontFamily: 'Inter_400Regular', fontSize: 14, paddingVertical: 12, color: '#142033' },
  error: { fontFamily: 'Inter_400Regular', fontSize: 13, lineHeight: 18, color: '#a30f2d' },
  help: { flexDirection: 'row', alignItems: 'flex-start', gap: 8, paddingHorizontal: 12, paddingVertical: 10, borderRadius: 12, backgroundColor: '#f1f7fc' },
  helpText: { flex: 1, minWidth: 0, fontFamily: 'Inter_400Regular', fontSize: 11, lineHeight: 16, color: '#444' },
  notice: { fontFamily: 'Inter_400Regular', fontSize: 13, lineHeight: 18, color: '#8b1a2e' },
  primary: { minHeight: 52, paddingHorizontal: 24, paddingVertical: 16, alignItems: 'center', justifyContent: 'center', borderRadius: 14, backgroundColor: '#8b1a2e', boxShadow: '0px 6px 16px rgba(139,26,46,0.25)' },
  primaryText: { fontFamily: 'Inter_600SemiBold', color: '#fff', textAlign: 'center' },
  loginRow: { flexDirection: 'row', flexWrap: 'wrap', alignItems: 'center', justifyContent: 'center', gap: 4 },
  loginLink: { minHeight: layout.touchTarget, minWidth: layout.touchTarget, justifyContent: 'center' },
  link: { fontFamily: 'Inter_500Medium', fontSize: 13, lineHeight: 18, color: '#a30f2d' },
  country: { marginTop: 'auto', paddingTop: 24, fontFamily: 'Inter_400Regular', fontSize: 9, lineHeight: 13, letterSpacing: 4, color: '#666', textAlign: 'center' },
});
