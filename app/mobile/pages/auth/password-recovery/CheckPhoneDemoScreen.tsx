import { useEffect, useRef, useState } from 'react';
import { StatusBar } from 'expo-status-bar';
import {
  Image,
  Keyboard,
  Pressable,
  ScrollView,
  StyleSheet,
  Text,
  TextInput,
  View,
  useWindowDimensions,
} from 'react-native';

import { Screen } from '../../../components/common/Screen';
import { layout, typography } from '../../../layout';
import { loginAssets } from '../LoginScreen';
import { registerAssets } from '../registration/RegisterScreen';

const phone = require('../../../assets/figma/register/phone.png');
const message = require('../../../assets/figma/chat-home/message.png');
type Props = { onBack: () => void; onDifferentNic: () => void; onLogin: () => void };
type Viewport = { width: number; height: number };

export function CheckPhoneDemoScreen(props: Props) {
  if (!__DEV__ || process.env.EXPO_PUBLIC_CONNECTED_UI_DEMO !== '1') return null;
  return <CheckPhoneForm {...props} />;
}

// Present the local six-digit demo without sending or verifying a code.

function CheckPhoneForm({ onBack, onDifferentNic, onLogin }: Props) {
  const [code, setCode] = useState('');
  const [submitted, setSubmitted] = useState(false);
  const [inputError, setInputError] = useState('');
  const [notice, setNotice] = useState('');
  const [focused, setFocused] = useState(false);
  const [cursor, setCursor] = useState(0);
  const selection = useRef({ start: 0, end: 0 });
  const transitioning = useRef(false);
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
  const error =
    inputError || (submitted && code.length !== 6 ? 'Enter all six digits for this demo.' : '');
  const contentWidth =
    closedViewport.width || Math.min(layout.contentWidth, Math.max(0, width - 32));
  const logoWidth = Math.min(150, Math.max(0, contentWidth - 198));
  const logoHeight = (logoWidth * 151.531) / 150;
  const headerGap = Math.max(
    12,
    Math.min(16, closedViewport.height - (cardHeight || 430 * fontScale) - logoHeight - 31),
  );
  const visionTop = Math.max(6, Math.min(32, 6 + (logoHeight - visionHeight) / 2));
  const headerHeight = Math.max(6 + logoHeight, visionTop + visionHeight, layout.touchTarget + 5);
  const artworkScale = Math.min(width / 400, 1.25);

  // Cancel the pending keyboard transition before accepting another layout phase.

  function clearTimer() {
    if (transitionTimer.current !== null) clearTimeout(transitionTimer.current);
    transitionTimer.current = null;
  }

  // Coalesce layout events without treating keyboard resizing as a new baseline.

  function scheduleLayout() {
    if (layoutFrame.current !== null) cancelAnimationFrame(layoutFrame.current);
    layoutFrame.current = requestAnimationFrame(() => {
      layoutFrame.current = requestAnimationFrame(() => {
        layoutFrame.current = null;
        const current = liveViewport.current,
          previous = baseline.current;
        const widthChanged = previous.width > 0 && Math.abs(current.width - previous.width) >= 0.5;
        if (
          !widthChanged &&
          (Keyboard.isVisible() || keyboardTop.current !== null || phase.current === 'opening')
        )
          return;
        if (phase.current === 'closing') {
          if (!widthChanged && current.height < previous.height - 0.5) return;
          phase.current = 'idle';
          clearTimer();
        }
        if (
          current.width > 0 &&
          (Math.abs(current.width - previous.width) >= 0.5 ||
            Math.abs(current.height - previous.height) >= 0.5)
        ) {
          baseline.current = current;
          setClosedViewport(current);
        }
      });
    });
  }

  // Reveal the focused field and its error using live window and keyboard bounds.

  function adjustVisibility() {
    const top = keyboardTop.current;
    if (top === null) return;
    viewport.current?.measureInWindow((_x, y, _w, height) => {
      if (keyboardTop.current !== top) return;
      const bottom = Math.min(y + height, top);
      const overlap = Math.max(0, y + height - top);
      setKeyboardOverlap(previous => (Math.abs(previous - overlap) < 0.5 ? previous : overlap));
      if (!fieldFocused.current) return;
      fieldGroup.current?.measureInWindow((_fieldX, fieldY, _fieldWidth, fieldHeight) => {
        if (keyboardTop.current !== top || !fieldFocused.current) return;
        const delta =
          fieldHeight > bottom - y - 24
            ? fieldY - (y + 12)
            : fieldY < y + 12
              ? fieldY - (y + 12)
              : fieldY + fieldHeight > bottom - 12
                ? fieldY + fieldHeight - (bottom - 12)
                : 0;
        if (Math.abs(delta) > 1)
          scroll.current?.scrollTo({ y: Math.max(0, offset.current + delta), animated: false });
      });
    });
  }

  // Wait for native layout before measuring the focused field against the keyboard.

  function scheduleVisibility() {
    if (visibilityFrame.current !== null) cancelAnimationFrame(visibilityFrame.current);
    visibilityFrame.current = requestAnimationFrame(() => {
      visibilityFrame.current = requestAnimationFrame(() => {
        visibilityFrame.current = null;
        adjustVisibility();
      });
    });
  }
  useEffect(() => {
    const show = Keyboard.addListener('keyboardDidShow', event => {
      clearTimer();
      phase.current = 'open';
      keyboardTop.current = event.endCoordinates.screenY;
      scheduleLayout();
      scheduleVisibility();
    });
    const hide = Keyboard.addListener('keyboardDidHide', () => {
      keyboardTop.current = null;
      phase.current = 'closing';
      setKeyboardOverlap(0);
      if (visibilityFrame.current !== null) cancelAnimationFrame(visibilityFrame.current);
      visibilityFrame.current = null;
      clearTimer();
      transitionTimer.current = setTimeout(() => {
        transitionTimer.current = null;
        phase.current = 'idle';
        scheduleLayout();
      }, 250);
      scheduleLayout();
    });
    const frame = Keyboard.addListener('keyboardDidChangeFrame', event => {
      if (!Keyboard.isVisible() && keyboardTop.current === null) return;
      keyboardTop.current = event.endCoordinates.screenY;
      scheduleVisibility();
    });
    const mountedInput = input.current;
    return () => {
      show.remove();
      hide.remove();
      frame.remove();
      clearTimer();
      if (layoutFrame.current !== null) cancelAnimationFrame(layoutFrame.current);
      if (visibilityFrame.current !== null) cancelAnimationFrame(visibilityFrame.current);
      mountedInput?.clear();
      Keyboard.dismiss();
    };
  }, []);
  useEffect(() => {
    scheduleVisibility();
  }, [error]);

  // Track focus while allowing time for keyboard metrics and native layout to arrive.

  function focus() {
    fieldFocused.current = true;
    setFocused(true);
    const metrics = Keyboard.metrics();
    if (metrics) {
      clearTimer();
      phase.current = 'open';
      keyboardTop.current = metrics.screenY;
    } else if (!Keyboard.isVisible() && keyboardTop.current === null) {
      phase.current = 'opening';
      clearTimer();
      transitionTimer.current = setTimeout(() => {
        transitionTimer.current = null;
        if (phase.current === 'opening' && !Keyboard.isVisible()) {
          phase.current = 'idle';
          scheduleLayout();
        }
      }, 500);
    }
    scheduleVisibility();
  }

  // Release the focus guard so a hardware keyboard cannot freeze viewport updates.

  function blur() {
    fieldFocused.current = false;
    setFocused(false);
    if (phase.current === 'opening' && !Keyboard.isVisible()) {
      clearTimer();
      phase.current = 'idle';
      scheduleLayout();
    }
  }

  // Accept up to six digits and keep native text and cursor aligned after normalization.

  function changeCode(raw: string) {
    const next = raw.replace(/[\s-]/g, '');
    setNotice('');
    if (!/^[0-9]*$/.test(next) || next.length > 6) {
      input.current?.setNativeProps({ text: code, selection: selection.current });
      setInputError('Use up to six digits for this demo. Letters are not accepted.');
      return;
    }
    setCode(next);
    setInputError('');
    if (raw !== next) {
      selection.current = { start: next.length, end: next.length };
      setCursor(next.length);
      input.current?.setNativeProps({ text: next, selection: selection.current });
    }
  }

  // Clear demo input and ignore repeated exits before changing screens.

  function leave(destination: () => void) {
    if (transitioning.current) return;
    transitioning.current = true;
    input.current?.clear();
    setCode('');
    setSubmitted(false);
    setInputError('');
    setNotice('');
    setFocused(false);
    fieldFocused.current = false;
    selection.current = { start: 0, end: 0 };
    setCursor(0);
    Keyboard.dismiss();
    destination();
  }

  // Validate the demo code locally while keeping verification disconnected.

  function verify() {
    if (transitioning.current) return;
    setSubmitted(true);
    setNotice('');
    if (code.length !== 6 || inputError) {
      input.current?.focus();
      return;
    }
    Keyboard.dismiss();
    setNotice('Verification is not connected. No code was verified.');
  }
  return (
    <View style={styles.root}>
      <StatusBar style="dark" />
      <View
        pointerEvents="none"
        accessible={false}
        importantForAccessibility="no-hide-descendants"
        style={StyleSheet.absoluteFill}
      >
        <Image
          source={loginAssets.topRibbon}
          accessible={false}
          resizeMode="cover"
          style={[styles.topRibbon, { width: 272 * artworkScale, height: 296 * artworkScale }]}
        />
        <Image
          source={loginAssets.skyline}
          accessible={false}
          resizeMode="contain"
          style={[styles.skyline, { width: 723 * artworkScale, height: 451 * artworkScale }]}
        />
        <Image
          source={loginAssets.bottomRibbon}
          accessible={false}
          resizeMode="cover"
          style={[styles.bottomRibbon, { width: 237 * artworkScale, height: 227 * artworkScale }]}
        />
      </View>
      <Screen scroll={false} horizontalPadding={16} contentStyle={styles.screen}>
        <View
          ref={viewport}
          style={styles.screen}
          onLayout={({ nativeEvent: { layout: measured } }) => {
            liveViewport.current = { width: measured.width, height: measured.height };
            scheduleLayout();
            scheduleVisibility();
          }}
        >
          <ScrollView
            ref={scroll}
            showsVerticalScrollIndicator={false}
            showsHorizontalScrollIndicator={false}
            alwaysBounceVertical={false}
            overScrollMode="never"
            keyboardShouldPersistTaps="handled"
            keyboardDismissMode="on-drag"
            automaticallyAdjustKeyboardInsets={false}
            automaticallyAdjustContentInsets={false}
            contentInsetAdjustmentBehavior="never"
            onScroll={event => {
              offset.current = event.nativeEvent.contentOffset.y;
            }}
            scrollEventThrottle={16}
            onContentSizeChange={scheduleVisibility}
            contentContainerStyle={[styles.content, { paddingBottom: keyboardOverlap + 8 }]}
          >
            <View style={[styles.header, { height: headerHeight }]}>
              <Pressable
                onPress={() => leave(onBack)}
                accessibilityRole="button"
                accessibilityLabel="Back to Forgot Password"
                style={styles.back}
              >
                <View style={styles.backCircle}>
                  <Image source={registerAssets.back} accessible={false} style={styles.backIcon} />
                </View>
              </Pressable>
              <View
                accessible
                accessibilityRole="image"
                accessibilityLabel="GovAI. People, services, a smarter Sri Lanka."
                style={[styles.logoSlot, { width: logoWidth, height: logoHeight }]}
              >
                <Image
                  source={loginAssets.logo}
                  accessible={false}
                  resizeMode="contain"
                  style={{
                    position: 'absolute',
                    width: logoWidth * 1.5417,
                    height: logoHeight * 2.7132,
                    left: -logoWidth * 0.3013,
                    top: -logoHeight * 0.636,
                  }}
                />
              </View>
              <View
                style={[styles.vision, { top: visionTop }]}
                onLayout={event => {
                  const measured = event.nativeEvent.layout.height;
                  setVisionHeight(previous =>
                    Math.abs(previous - measured) < 0.5 ? previous : measured,
                  );
                }}
              >
                <Text style={styles.visionText}>{'FOR A\nSMARTER\nSRI LANKA'}</Text>
                <View style={styles.accent} />
              </View>
            </View>
            <View
              style={[styles.card, { marginTop: headerGap }]}
              onLayout={event => {
                const measured = event.nativeEvent.layout.height;
                setCardHeight(previous =>
                  Math.abs(previous - measured) < 0.5 ? previous : measured,
                );
              }}
            >
              <View style={styles.badge}>
                <Image source={phone} accessible={false} style={styles.phoneIcon} />
                <Image source={message} accessible={false} style={styles.messageIcon} />
              </View>
              <View style={styles.cardHeader}>
                <Text accessibilityRole="header" style={styles.heading}>
                  Check your phone
                </Text>
                <Text style={styles.description}>Demo preview — no SMS was sent.</Text>
                <Text style={styles.description}>Sample code destination:</Text>
                <Text style={styles.samplePhone}>+94 7••• ••12</Text>
              </View>
              <View ref={fieldGroup} onLayout={scheduleVisibility} style={styles.fieldGroup}>
                <Text style={styles.label}>Verification code</Text>
                <View style={styles.codeEntry}>
                  <View
                    pointerEvents="none"
                    accessible={false}
                    importantForAccessibility="no-hide-descendants"
                    style={styles.slots}
                  >
                    {Array.from({ length: 6 }, (_, index) => (
                      <View
                        key={index}
                        style={[
                          styles.slot,
                          focused && Math.min(cursor, 5) === index && styles.activeSlot,
                          !!error && styles.errorSlot,
                        ]}
                      >
                        <Text style={styles.digit}>{code[index] || ' '}</Text>
                      </View>
                    ))}
                  </View>
                  <TextInput
                    ref={input}
                    value={code}
                    onChangeText={changeCode}
                    keyboardType="number-pad"
                    inputMode="numeric"
                    accessibilityLabel="Verification code. Six digits for this demo only."
                    accessibilityHint="One numeric field shown as six slots. Verification is not connected. Use sample digits."
                    autoCapitalize="none"
                    autoCorrect={false}
                    spellCheck={false}
                    autoComplete="off"
                    importantForAutofill="no"
                    caretHidden
                    returnKeyType="done"
                    onSubmitEditing={verify}
                    onFocus={focus}
                    onBlur={blur}
                    onSelectionChange={({ nativeEvent: { selection: next } }) => {
                      selection.current = next;
                      setCursor(next.start);
                    }}
                    style={styles.nativeInput}
                  />
                </View>
                {!!error && (
                  <Text accessibilityLiveRegion="polite" style={styles.error}>
                    {error}
                  </Text>
                )}
                <Text style={styles.caption}>
                  Six digits are a demo format; backend format is unconfirmed.
                </Text>
              </View>
              <Pressable
                onPress={verify}
                accessibilityRole="button"
                accessibilityLabel="Verify code"
                accessibilityHint="Local format check only; no code will be authenticated."
                style={styles.primary}
              >
                <Text style={[typography.action, styles.primaryText]}>Verify code</Text>
                <Image source={registerAssets.next} accessible={false} style={styles.nextIcon} />
              </Pressable>
              <Text style={styles.caption}>Verification is not connected.</Text>
              {!!notice && (
                <Text accessibilityLiveRegion="polite" style={styles.notice}>
                  {notice}
                </Text>
              )}
              <View style={styles.footer}>
                <View style={styles.resendRow}>
                  <Text style={styles.description}>Didn’t get it?</Text>
                  <Pressable
                    disabled
                    accessibilityRole="link"
                    accessibilityState={{ disabled: true }}
                    accessibilityLabel="Resend code. Unavailable in demo; no SMS will be sent."
                    style={styles.linkTarget}
                  >
                    <Text style={styles.link}>Resend code</Text>
                  </Pressable>
                </View>
                <Text style={styles.caption}>Unavailable in demo</Text>
                <Pressable
                  onPress={() => leave(onLogin)}
                  accessibilityRole="link"
                  accessibilityLabel="Back to log in"
                  style={styles.linkTarget}
                >
                  <Text style={styles.link}>Back to log in</Text>
                </Pressable>
                <Pressable
                  onPress={() => leave(onDifferentNic)}
                  accessibilityRole="link"
                  accessibilityLabel="Use a different NIC"
                  style={styles.linkTarget}
                >
                  <Text style={styles.link}>Use a different NIC</Text>
                </Pressable>
              </View>
            </View>
            <Text style={styles.country}>SRI LANKA</Text>
          </ScrollView>
        </View>
      </Screen>
    </View>
  );
}

const styles = StyleSheet.create({
  root: { flex: 1, backgroundColor: '#fff' },
  screen: { flex: 1 },
  content: { flexGrow: 1 },
  topRibbon: { position: 'absolute', left: -9, top: -14 },
  skyline: { position: 'absolute', left: -164, bottom: -6, opacity: 0.45 },
  bottomRibbon: { position: 'absolute', right: -7, bottom: -8 },
  header: { alignItems: 'center', width: '100%', flexShrink: 0 },
  back: {
    position: 'absolute',
    left: 0,
    top: 5,
    width: 48,
    height: 48,
    alignItems: 'center',
    justifyContent: 'center',
    zIndex: 2,
  },
  backCircle: {
    width: 38,
    height: 38,
    borderRadius: 9999,
    backgroundColor: '#fff',
    alignItems: 'center',
    justifyContent: 'center',
  },
  backIcon: { width: 20, height: 20 },
  logoSlot: { marginTop: 6, overflow: 'hidden' },
  vision: { position: 'absolute', right: 9, width: 86, alignItems: 'flex-end', gap: 6 },
  visionText: {
    fontFamily: 'Inter_500Medium',
    color: '#666',
    fontSize: 9,
    lineHeight: 13,
    letterSpacing: 2,
    textAlign: 'right',
  },
  accent: { width: 36, height: 2, backgroundColor: '#f5a915' },
  card: {
    width: '100%',
    flexShrink: 0,
    paddingHorizontal: 16,
    paddingVertical: 8,
    gap: 4,
    alignItems: 'center',
    borderRadius: 24,
    borderWidth: 1,
    borderColor: '#f1f5f9',
    backgroundColor: '#fff',
    boxShadow: '0px 10px 22px rgba(0,0,0,0.1)',
  },
  badge: {
    width: 48,
    height: 48,
    borderRadius: 24,
    backgroundColor: '#edf6f1',
    alignItems: 'center',
    justifyContent: 'center',
  },
  phoneIcon: { width: 26, height: 26, tintColor: '#007a53' },
  messageIcon: {
    position: 'absolute',
    width: 16,
    height: 16,
    right: 6,
    top: 6,
    tintColor: '#007a53',
    backgroundColor: '#edf6f1',
  },
  cardHeader: { width: '100%', gap: 2, alignItems: 'center' },
  heading: {
    fontFamily: 'Inter_700Bold',
    fontSize: 20,
    lineHeight: 28,
    color: '#142033',
    textAlign: 'center',
  },
  description: {
    fontFamily: 'Inter_400Regular',
    fontSize: 13,
    lineHeight: 18,
    color: '#666',
    textAlign: 'center',
  },
  samplePhone: {
    fontFamily: 'Inter_700Bold',
    fontSize: 15,
    lineHeight: 21,
    color: '#142033',
    textAlign: 'center',
  },
  fieldGroup: { width: '100%', gap: 4 },
  label: { fontFamily: 'Inter_500Medium', fontSize: 13, lineHeight: 18, color: '#666' },
  codeEntry: { width: '100%', minHeight: 48, position: 'relative' },
  slots: { flexDirection: 'row', gap: 6 },
  slot: {
    flex: 1,
    minWidth: 0,
    minHeight: 48,
    paddingVertical: 8,
    borderWidth: 1,
    borderColor: '#cbd5e1',
    borderRadius: 8,
    alignItems: 'center',
    justifyContent: 'center',
  },
  activeSlot: { borderColor: '#a30f2d', borderWidth: 2 },
  errorSlot: { borderColor: '#a30f2d' },
  digit: { fontFamily: 'Inter_500Medium', fontSize: 20, lineHeight: 28, color: '#142033' },
  // Keep the one native field full-size for touch, editing and accessibility, but
  // make its duplicate glyph layer imperceptible even when Android overrides a
  // transparent text color. The six slot Text nodes remain the visible value.

  nativeInput: {
    position: 'absolute',
    top: 0,
    bottom: 0,
    left: 0,
    right: 0,
    minHeight: 48,
    minWidth: 48,
    padding: 0,
    color: 'rgba(0,0,0,0)',
    backgroundColor: 'transparent',
    opacity: 0.01,
    fontSize: 1,
    lineHeight: 1,
    letterSpacing: -1,
  },
  error: { fontFamily: 'Inter_400Regular', fontSize: 13, lineHeight: 18, color: '#a30f2d' },
  caption: {
    fontFamily: 'Inter_400Regular',
    fontSize: 11,
    lineHeight: 16,
    color: '#666',
    textAlign: 'center',
  },
  primary: {
    width: '100%',
    minHeight: 48,
    paddingHorizontal: 20,
    paddingVertical: 12,
    flexDirection: 'row',
    flexWrap: 'wrap',
    alignItems: 'center',
    justifyContent: 'center',
    gap: 10,
    borderRadius: 14,
    backgroundColor: '#8b1730',
    boxShadow: '0px 6px 16px rgba(139,23,48,0.2)',
  },
  primaryText: {
    flexShrink: 1,
    fontFamily: 'Inter_600SemiBold',
    color: '#fff',
    textAlign: 'center',
  },
  nextIcon: { width: 18, height: 18 },
  footer: { width: '100%', gap: 2, alignItems: 'center' },
  resendRow: {
    flexDirection: 'row',
    flexWrap: 'wrap',
    justifyContent: 'center',
    alignItems: 'center',
    columnGap: 4,
  },
  linkTarget: { minHeight: 48, minWidth: 48, justifyContent: 'center', maxWidth: '100%' },
  link: {
    fontFamily: 'Inter_500Medium',
    fontSize: 13,
    lineHeight: 18,
    color: '#a30f2d',
    textDecorationLine: 'underline',
    textAlign: 'center',
  },
  notice: {
    fontFamily: 'Inter_400Regular',
    fontSize: 13,
    lineHeight: 18,
    color: '#8b1730',
    textAlign: 'center',
  },
  country: {
    marginTop: 'auto',
    paddingTop: 4,
    fontFamily: 'Inter_400Regular',
    fontSize: 9,
    lineHeight: 13,
    letterSpacing: 4,
    color: '#666',
    textAlign: 'center',
  },
});
