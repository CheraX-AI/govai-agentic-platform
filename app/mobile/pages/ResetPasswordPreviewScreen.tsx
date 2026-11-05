import { useEffect, useRef, useState } from 'react';
import { StatusBar } from 'expo-status-bar';
import { BackHandler, Image, Keyboard, Pressable, ScrollView, StyleSheet, Text, TextInput, View, useWindowDimensions } from 'react-native';
import { Screen } from '../components/Screen';
import { layout, typography } from '../layout';
import { loginAssets } from './LoginScreen';
import { registerAssets } from './RegisterScreen';

const disconnectedNotice = 'Password recovery is not connected. No request was sent and no password was changed.';
type Viewport = { width: number; height: number };

export function ResetPasswordPreviewScreen({ onExit }: { onExit: () => void }) {
  if (!__DEV__ || process.env.EXPO_PUBLIC_RESET_PASSWORD_PREVIEW !== '1') return null;
  return <ResetPasswordForm onExit={onExit} />;
}

function ResetPasswordForm({ onExit }: { onExit: () => void }) {
  const [values, setValues] = useState(['', '']);
  const [revealed, setRevealed] = useState([false, false]);
  const [submitted, setSubmitted] = useState(false);
  const [notice, setNotice] = useState('');
  const [closedViewport, setClosedViewport] = useState<Viewport>({ width: 0, height: 0 });
  const [cardHeight, setCardHeight] = useState(0);
  const [visionHeight, setVisionHeight] = useState(47);
  const [keyboardOverlap, setKeyboardOverlap] = useState(0);
  const viewport = useRef<View>(null);
  const groups = useRef<Array<View | null>>([]);
  const inputs = useRef<Array<TextInput | null>>([]);
  const scroll = useRef<ScrollView>(null);
  const liveViewport = useRef<Viewport>({ width: 0, height: 0 });
  const baseline = useRef<Viewport>({ width: 0, height: 0 });
  const keyboardTop = useRef<number | null>(null);
  const phase = useRef<'idle' | 'opening' | 'open' | 'closing'>('idle');
  const fieldFocused = useRef<number | null>(null);
  const offset = useRef(0);
  const layoutFrame = useRef<number | null>(null);
  const visibilityFrame = useRef<number | null>(null);
  const transitionTimer = useRef<ReturnType<typeof setTimeout> | null>(null);
  const { width, fontScale } = useWindowDimensions();
  const errors = submitted ? [values[0].length === 0 ? 'Enter a new password.' : '',
    values[1].length === 0 ? 'Confirm your new password.' : values[0] !== values[1] ? 'Passwords do not match.' : ''] : ['', ''];
  const contentWidth = closedViewport.width || Math.min(layout.contentWidth, Math.max(0, width - 32));
  const maximumLogoWidth = Math.min(118, Math.max(0, contentWidth - 2 * (86 + 9 + 4)));
  const maximumLogoHeight = maximumLogoWidth * 151.531 / 150;
  const intrinsicCardHeight = cardHeight || 550 * fontScale;
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
      if (fieldFocused.current === null) return;
      groups.current[fieldFocused.current]?.measureInWindow((_fieldX, fieldY, _fieldWidth, fieldHeight) => {
        if (keyboardTop.current !== top || fieldFocused.current === null) return;
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
    const mountedInputs = [...inputs.current];
    return () => {
      show.remove(); hide.remove(); frame.remove(); clearTimer();
      if (layoutFrame.current !== null) cancelAnimationFrame(layoutFrame.current);
      if (visibilityFrame.current !== null) cancelAnimationFrame(visibilityFrame.current);
      mountedInputs.forEach(input => input?.clear()); Keyboard.dismiss();
    };
  }, []);
  useEffect(() => { scheduleVisibility(); }, [errors[0], errors[1]]);
  function focus(index: number) {
    fieldFocused.current = index;
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
    fieldFocused.current = null;
    if (phase.current === 'opening' && !Keyboard.isVisible()) { clearTimer(); phase.current = 'idle'; scheduleLayout(); }
  }
  function leave() {
    inputs.current.forEach(input => input?.clear());
    setValues(['', '']); setRevealed([false, false]); setSubmitted(false); setNotice(''); fieldFocused.current = null;
    Keyboard.dismiss(); onExit();
  }
  useEffect(() => {
    const subscription = BackHandler.addEventListener('hardwareBackPress', () => {
      if (Keyboard.isVisible()) { Keyboard.dismiss(); return true; }
      leave(); return true;
    });
    return () => subscription.remove();
  }, [onExit]);
  function submit() {
    setSubmitted(true); setNotice('');
    const invalid = values[0].length === 0 ? 0 : values[1].length === 0 || values[0] !== values[1] ? 1 : -1;
    if (invalid >= 0) { inputs.current[invalid]?.focus(); return; }
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
        <ScrollView ref={scroll} showsVerticalScrollIndicator={false} showsHorizontalScrollIndicator={false} alwaysBounceVertical={false} overScrollMode="never"
          keyboardShouldPersistTaps="handled" keyboardDismissMode="on-drag" automaticallyAdjustKeyboardInsets={false}
          automaticallyAdjustContentInsets={false} contentInsetAdjustmentBehavior="never"
          onScroll={event => { offset.current = event.nativeEvent.contentOffset.y; }} scrollEventThrottle={16}
          onContentSizeChange={scheduleVisibility} contentContainerStyle={[styles.content, { paddingBottom: keyboardOverlap + 8 }]}>
          <View style={[styles.header, { height: headerHeight }]}>
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
            <Text style={styles.previewNotice}>Disconnected development preview. Use sample passwords only.</Text>
            <View style={styles.cardHeader}>
              <Text accessibilityRole="header" style={styles.heading}>Choose a new password</Text>
              <Text style={styles.description}>Use at least 8 characters, and don’t reuse a password from another site.</Text>
            </View>
            {[0, 1].map(index => <View key={index} style={styles.fieldGroup}>
              {index === 1 && <View style={styles.guidance}>
                <View style={styles.strength} accessible accessibilityLabel="Strength not evaluated">
                  <View style={styles.bars}>{[0, 1, 2, 3].map(bar => <View key={bar} style={styles.bar} />)}</View>
                  <Text style={styles.small}>Strength not evaluated</Text>
                </View>
                <View style={styles.requirements}>
                  <Text style={styles.small}>Design guidance — pending backend confirmation</Text>
                  {['At least 8 characters', 'Upper and lower case letters', 'At least one number', 'At least one symbol, like ! @ # $'].map(rule =>
                    <View key={rule} style={styles.rule}><View accessible={false} style={styles.neutralDot} /><Text style={styles.ruleText}>{rule}</Text></View>)}
                </View>
              </View>}
              <View ref={node => { groups.current[index] = node; }} onLayout={scheduleVisibility} style={styles.fieldGroup}>
                <Text style={styles.label}>{index === 0 ? 'New password' : 'Confirm new password'}</Text>
                <View style={[styles.field, !!errors[index] && styles.emphasized]}>
                  <Image source={registerAssets.lock} accessible={false} style={styles.identityIcon} />
                  <TextInput ref={node => { inputs.current[index] = node; }} value={values[index]}
                    onChangeText={value => { setValues(previous => previous.map((old, i) => i === index ? value : old)); setNotice(''); }}
                    accessibilityLabel={(index === 0 ? 'New password' : 'Confirm new password') + '. Required.' + (errors[index] ? ' ' + errors[index] : '')}
                    autoCapitalize="none" autoCorrect={false} spellCheck={false} autoComplete="off" importantForAutofill="no"
                    secureTextEntry={!revealed[index]} returnKeyType={index === 0 ? 'next' : 'done'} submitBehavior="submit"
                    onSubmitEditing={() => index === 0 ? inputs.current[1]?.focus() : submit()}
                    onFocus={() => focus(index)} onBlur={blur} style={styles.input} />
                  <Pressable accessibilityRole="button" accessibilityLabel={(revealed[index] ? 'Hide' : 'Show') + (index === 0 ? ' new password' : ' confirmation password')}
                    accessibilityState={{ selected: revealed[index] }} style={styles.eye}
                    onPress={() => { setRevealed(previous => previous.map((old, i) => i === index ? !old : old)); scheduleVisibility(); }}>
                    <Image source={revealed[index] ? loginAssets.eye : registerAssets.eyeOff} accessible={false} style={styles.identityIcon} />
                  </Pressable>
                </View>
                {!!errors[index] && <Text accessibilityLiveRegion="polite" style={styles.error}>{errors[index]}</Text>}
              </View>
            </View>)}
            {!!notice && <Text accessibilityLiveRegion="polite" style={styles.notice}>{notice}</Text>}
            <Pressable onPress={submit} accessibilityRole="button" accessibilityLabel="Reset password. Validate sample entries only; password recovery is disconnected." style={styles.primary}>
              <Text style={[typography.action, styles.primaryText]}>Reset password</Text>
            </Pressable>
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
  logoSlot: { marginTop: 6, overflow: 'hidden' },
  vision: { position: 'absolute', right: 9, width: 86, alignItems: 'flex-end', gap: 6 },
  visionText: { fontFamily: 'Inter_500Medium', color: '#666', fontSize: 9, lineHeight: 13, letterSpacing: 2, textAlign: 'right' },
  accent: { width: 36, height: 2, backgroundColor: '#f5a915' },
  card: { width: '100%', flexShrink: 0, paddingHorizontal: 20, paddingVertical: 24, gap: 16, borderRadius: 24, borderWidth: 1, borderColor: '#f1f5f9', backgroundColor: '#fff', boxShadow: '0px 10px 22px rgba(0,0,0,0.1)' },
  identityIcon: { width: 18, height: 18 },
  cardHeader: { gap: 6 }, heading: { fontFamily: 'Inter_700Bold', fontSize: 20, lineHeight: 28, color: '#142033' },
  description: { fontFamily: 'Inter_400Regular', fontSize: 13, lineHeight: 18, color: '#666' },
  fieldGroup: { gap: 6 }, label: { fontFamily: 'Inter_500Medium', fontSize: 13, lineHeight: 18, color: '#142033' },
  field: { minHeight: layout.touchTarget, flexDirection: 'row', alignItems: 'center', gap: 10, paddingLeft: 14, paddingRight: 4, borderRadius: 12, borderWidth: 1.5, borderColor: '#e5e7eb', backgroundColor: '#fff', boxShadow: '0px 1px 4px rgba(0,0,0,0.04)' },
  emphasized: { borderColor: '#a30f2d' }, input: { flex: 1, minWidth: 0, fontFamily: 'Inter_400Regular', fontSize: 14, paddingVertical: 12, color: '#142033' },
  eye: { width: layout.touchTarget, minHeight: layout.touchTarget, justifyContent: 'center', alignItems: 'center' },
  previewNotice: { fontFamily: 'Inter_400Regular', fontSize: 12, lineHeight: 17, color: '#666' },
  guidance: { gap: 12, marginBottom: 10 }, strength: { gap: 8, flexDirection: 'row', flexWrap: 'wrap', alignItems: 'center' },
  bars: { flexDirection: 'row', gap: 4, flexGrow: 1, minWidth: 100 }, bar: { flex: 1, height: 4, borderRadius: 2, backgroundColor: '#d1d5db' },
  small: { fontFamily: 'Inter_400Regular', fontSize: 11, lineHeight: 16, color: '#595959' },
  requirements: { gap: 8, padding: 12, borderRadius: 12, backgroundColor: '#f7f9fb' },
  rule: { flexDirection: 'row', gap: 8, alignItems: 'flex-start' }, neutralDot: { width: 14, height: 14, borderRadius: 7, borderWidth: 1, borderColor: '#9ca3af', marginTop: 1 },
  ruleText: { flex: 1, minWidth: 0, fontFamily: 'Inter_400Regular', fontSize: 11, lineHeight: 16, color: '#595959' },
  error: { fontFamily: 'Inter_400Regular', fontSize: 13, lineHeight: 18, color: '#a30f2d' },
  notice: { fontFamily: 'Inter_400Regular', fontSize: 13, lineHeight: 18, color: '#8b1a2e' },
  primary: { minHeight: 52, paddingHorizontal: 24, paddingVertical: 16, alignItems: 'center', justifyContent: 'center', borderRadius: 14, backgroundColor: '#8b1a2e', boxShadow: '0px 6px 16px rgba(139,26,46,0.25)' },
  primaryText: { fontFamily: 'Inter_600SemiBold', color: '#fff', textAlign: 'center' },
  country: { marginTop: 'auto', paddingTop: 24, fontFamily: 'Inter_400Regular', fontSize: 9, lineHeight: 13, letterSpacing: 4, color: '#666', textAlign: 'center' },
});
