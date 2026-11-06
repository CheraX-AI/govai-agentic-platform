import { randomUUID } from 'expo-crypto';
import { loadMobileApiConfig } from '../../../services/apiConfig';
import { registerCitizen } from '../../../services/registrationApi';
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

export const registerAssets = {
  back: require('../../../assets/figma/register/back.png'),
  user: require('../../../assets/figma/register/user.png'),
  identity: require('../../../assets/figma/register/identity.png'),
  mail: require('../../../assets/figma/register/mail.png'),
  phone: require('../../../assets/figma/register/phone.png'),
  lock: require('../../../assets/figma/register/lock.png'),
  eyeOff: require('../../../assets/figma/register/eye-off.png'),
  next: require('../../../assets/figma/register/next.png'),
};

const fields = [
  {
    key: 'fullName',
    label: 'Full name',
    placeholder: 'Full name as on your NIC',
    icon: registerAssets.user,
  },
  {
    key: 'nic',
    label: 'National ID number',
    placeholder: '200012345678 or 991234567V',
    icon: registerAssets.identity,
  },
  { key: 'email', label: 'Email', placeholder: 'name@gmail.com', icon: registerAssets.mail },
  { key: 'phone', label: 'Phone number', placeholder: '07X XXX XXXX', icon: registerAssets.phone },
  {
    key: 'password',
    label: 'Password',
    placeholder: 'Create a password',
    icon: registerAssets.lock,
  },
  {
    key: 'confirmPassword',
    label: 'Confirm password',
    placeholder: 'Re-enter your password',
    icon: registerAssets.lock,
  },
] as const;
type Field = (typeof fields)[number]['key'];
type Values = Record<Field, string>;
const emptyValues = (): Values => ({
  fullName: '',
  nic: '',
  email: '',
  phone: '',
  password: '',
  confirmPassword: '',
});
const requiredMessages: Record<Field, string> = {
  fullName: 'Enter your full name.',
  nic: 'Enter your National ID number.',
  email: 'Enter your email.',
  phone: 'Enter your phone number.',
  password: 'Enter a password.',
  confirmPassword: 'Re-enter your password.',
};

// Validate the local registration form and gate the optional development demo.

export function RegisterScreen({
  onLogin,
  onDemoValidated,
}: {
  onLogin: () => void;
  onDemoValidated?: () => void;
}) {
  const transitioning = useRef(false);
  const mounted = useRef(true);
  const operation = useRef<string | null>(null);
  const [pending, setPending] = useState(false);
  const [received, setReceived] = useState(false);
  useEffect(() => {
    mounted.current = true;
    return () => {
      mounted.current = false;
    };
  }, []);
  // Credentials remain transient in this mounted screen.

  const [values, setValues] = useState<Values>(emptyValues);
  const [submitted, setSubmitted] = useState(false);
  const [agreement, setAgreement] = useState(false);
  const [revealed, setRevealed] = useState({ password: false, confirmPassword: false });
  const [notice, setNotice] = useState(
    'Use your email and an international phone number, including country code.',
  );
  const [viewport, setViewport] = useState({ width: 0, height: 0 });
  const [formHeight, setFormHeight] = useState(0);
  const [visionHeight, setVisionHeight] = useState(47);
  const [keyboardOverlap, setKeyboardOverlap] = useState(0);
  const { width, fontScale } = useWindowDimensions();
  const inputRefs = useRef<Partial<Record<Field, TextInput>>>({});
  const groupRefs = useRef<Partial<Record<Field, View>>>({});
  const viewportRef = useRef<View>(null);
  const scroll = useRef<ScrollView>(null);
  const keyboardTop = useRef<number | null>(null);
  const focused = useRef<Field | null>(null);
  const scrollOffset = useRef(0);
  const adjustmentFrame = useRef<number | null>(null);
  const adjustmentVersion = useRef(0);
  const manualScroll = useRef(false);
  const artworkScale = Math.min(width / 400, 1.25);
  const contentWidth = viewport.width || Math.min(layout.contentWidth, Math.max(0, width - 32));
  // Intrinsic form/vision measurements never include flexible header/card space.
  // Start compact until measured, rather than flashing an oversized header.

  const maximumLogoHeight = Math.min(
    119.204,
    (Math.max(0, contentWidth - 2 * (86 + 4)) * 119.204) / 118,
  );
  const logoHeight = Math.min(
    maximumLogoHeight,
    Math.max(
      Math.min(64, maximumLogoHeight),
      viewport.height - (formHeight || 900 * fontScale) - 44 - 12,
    ),
  );
  const logoWidth = (logoHeight * 118) / 119.204;
  const visionTop = Math.max(0, Math.min(32, (logoHeight - visionHeight) / 2));
  const headerHeight = Math.max(layout.touchTarget, logoHeight, visionTop + visionHeight);
  const errors: Partial<Record<Field, string>> = {};
  if (submitted) {
    for (const field of fields) {
      // Passwords are never trimmed or transformed. Examples are not rules.

      const blank =
        field.key === 'password' || field.key === 'confirmPassword'
          ? values[field.key].length === 0
          : values[field.key].trim().length === 0;
      if (blank) errors[field.key] = requiredMessages[field.key];
    }
    if (
      values.password.length > 0 &&
      values.confirmPassword.length > 0 &&
      values.password !== values.confirmPassword
    ) {
      errors.confirmPassword = 'Passwords do not match.';
    }
  }

  // Reveal the focused field and its error using live window and keyboard bounds.

  function adjustVisibility(version: number) {
    const top = keyboardTop.current;
    const field = focused.current;
    if (top === null) return;
    viewportRef.current?.measureInWindow((_x, y, _width, height) => {
      if (keyboardTop.current !== top || adjustmentVersion.current !== version) return;
      const bottom = Math.min(y + height, top);
      const overlap = Math.max(0, y + height - top);
      setKeyboardOverlap(previous => (Math.abs(previous - overlap) < 0.5 ? previous : overlap));
      if (manualScroll.current || focused.current !== field) return;
      const group = field ? groupRefs.current[field] : undefined;
      group?.measureInWindow((_fieldX, fieldY, _fieldWidth, fieldHeight) => {
        if (
          keyboardTop.current !== top ||
          adjustmentVersion.current !== version ||
          manualScroll.current ||
          focused.current !== field
        )
          return;
        const delta =
          fieldHeight > bottom - y - 24
            ? fieldY - (y + 12)
            : fieldY < y + 12
              ? fieldY - (y + 12)
              : fieldY + fieldHeight > bottom - 12
                ? fieldY + fieldHeight - (bottom - 12)
                : 0;
        if (Math.abs(delta) > 1)
          scroll.current?.scrollTo({
            y: Math.max(0, scrollOffset.current + delta),
            animated: false,
          });
      });
    });
  }

  // Invalidate measurements already in flight as well as queued animation frames.

  function cancelVisibilityCheck() {
    adjustmentVersion.current++;
    if (adjustmentFrame.current !== null) cancelAnimationFrame(adjustmentFrame.current);
    adjustmentFrame.current = null;
  }

  // Keep manual control through drag and momentum until focus or keyboard intent changes.

  function pauseAutomaticScrolling() {
    manualScroll.current = true;
    cancelVisibilityCheck();
  }

  // Wait for native layout before measuring the focused field against the keyboard.

  function scheduleVisibilityCheck() {
    cancelVisibilityCheck();
    const version = adjustmentVersion.current;
    adjustmentFrame.current = requestAnimationFrame(() => {
      if (adjustmentVersion.current !== version) return;
      adjustmentFrame.current = requestAnimationFrame(() => {
        if (adjustmentVersion.current !== version) return;
        adjustmentFrame.current = null;
        adjustVisibility(version);
      });
    });
  }
  useEffect(() => {
    const show = Keyboard.addListener('keyboardDidShow', event => {
      keyboardTop.current = event.endCoordinates.screenY;
      manualScroll.current = false;
      scheduleVisibilityCheck();
    });
    const hide = Keyboard.addListener('keyboardDidHide', () => {
      keyboardTop.current = null;
      setKeyboardOverlap(0);
      cancelVisibilityCheck();
    });
    const changeFrame = Keyboard.addListener('keyboardDidChangeFrame', event => {
      if (!Keyboard.isVisible() && keyboardTop.current === null) return;
      keyboardTop.current = event.endCoordinates.screenY;
      scheduleVisibilityCheck();
    });
    // Capture mounted controls before React detaches callback refs on unmount.

    const mountedInputs = Object.values(inputRefs.current);
    return () => {
      show.remove();
      hide.remove();
      changeFrame.remove();
      cancelVisibilityCheck();
      mountedInputs.forEach(input => input?.clear());
      Keyboard.dismiss();
    };
  }, []);

  // Track the active field and schedule a visibility check against the keyboard.

  function focusField(key: Field) {
    focused.current = key;
    manualScroll.current = false;
    const metrics = Keyboard.metrics();
    if (metrics) keyboardTop.current = metrics.screenY;
    scheduleVisibilityCheck();
  }

  // Clear local form values and dismiss the keyboard before leaving the screen.

  function leave() {
    if (transitioning.current) return;
    Object.values(inputRefs.current).forEach(input => input?.clear());
    setValues(emptyValues());
    setSubmitted(false);
    setAgreement(false);
    setRevealed({ password: false, confirmPassword: false });
    Keyboard.dismiss();
    onLogin();
  }

  // Validate required fields before registration or the guarded demo.

  async function submit() {
    if (transitioning.current) return;
    if (received) {
      leave();
      return;
    }
    setSubmitted(true);
    const invalid = fields.find(field =>
      field.key === 'password' || field.key === 'confirmPassword'
        ? values[field.key].length === 0
        : values[field.key].trim().length === 0,
    );
    if (invalid) {
      inputRefs.current[invalid.key]?.focus();
      focusField(invalid.key);
      return;
    }
    if (values.password !== values.confirmPassword) {
      inputRefs.current.confirmPassword?.focus();
      focusField('confirmPassword');
      return;
    }
    if (__DEV__ && process.env.EXPO_PUBLIC_CONNECTED_UI_DEMO === '1' && onDemoValidated) {
      transitioning.current = true;
      Object.values(inputRefs.current).forEach(input => input?.clear());
      setValues(emptyValues());
      setSubmitted(false);
      setAgreement(false);
      setRevealed({ password: false, confirmPassword: false });
      Keyboard.dismiss();
      onDemoValidated();
      return;
    }
    // Agreement is optional: approved Terms and consent validation are pending.

    transitioning.current = true;
    setPending(true);
    Keyboard.dismiss();
    try {
      const config = loadMobileApiConfig();
      operation.current ??= randomUUID();
      await registerCitizen(
        config.authBaseUrl,
        {
          full_name: values.fullName.trim(),
          nic: values.nic.trim(),
          mobile_number: values.phone.trim(),
          email: values.email.trim(),
          password: values.password,
        },
        operation.current,
      );
      if (mounted.current) {
        setReceived(true);
        setNotice(
          'Request received. If eligible, check your email for the verification link, then return to Log in.',
        );
        setValues(emptyValues());
        setSubmitted(false);
        Object.values(inputRefs.current).forEach(input => input?.clear());
      }
    } catch (error) {
      if (mounted.current)
        setNotice(error instanceof Error ? error.message : 'Registration is unavailable.');
    } finally {
      transitioning.current = false;
      if (mounted.current) setPending(false);
    }
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
          resizeMode="cover"
          style={[styles.topRibbon, { width: 272 * artworkScale, height: 296 * artworkScale }]}
        />
        <Image
          source={loginAssets.skyline}
          resizeMode="contain"
          style={[styles.skyline, { width: 723 * artworkScale, height: 451 * artworkScale }]}
        />
        <Image
          source={loginAssets.bottomRibbon}
          resizeMode="cover"
          style={[styles.bottomRibbon, { width: 237 * artworkScale, height: 227 * artworkScale }]}
        />
        <Text style={styles.country}>SRI LANKA</Text>
      </View>
      <Screen scroll={false} horizontalPadding={16} contentStyle={styles.screen}>
        <View
          ref={viewportRef}
          style={styles.screen}
          onLayout={({ nativeEvent: { layout: measured } }) => {
            setViewport(previous =>
              Math.abs(previous.width - measured.width) < 0.5 &&
              Math.abs(previous.height - measured.height) < 0.5
                ? previous
                : { width: measured.width, height: measured.height },
            );
            scheduleVisibilityCheck();
          }}
        >
          <ScrollView
            ref={scroll}
            showsVerticalScrollIndicator={false}
            overScrollMode="never"
            alwaysBounceVertical={false}
            keyboardShouldPersistTaps="handled"
            keyboardDismissMode="on-drag"
            automaticallyAdjustKeyboardInsets={false}
            automaticallyAdjustContentInsets={false}
            contentInsetAdjustmentBehavior="never"
            onScrollBeginDrag={pauseAutomaticScrolling}
            onScroll={event => {
              scrollOffset.current = event.nativeEvent.contentOffset.y;
            }}
            scrollEventThrottle={16}
            onContentSizeChange={scheduleVisibilityCheck}
            contentContainerStyle={[styles.scrollContent, { paddingBottom: keyboardOverlap }]}
          >
            <View style={[styles.header, { height: headerHeight }]}>
              <Pressable
                onPress={leave}
                accessibilityRole="button"
                accessibilityLabel="Back to Login"
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
                  style={[
                    styles.logo,
                    {
                      width: logoWidth * 1.5417,
                      height: logoHeight * 2.7132,
                      left: -logoWidth * 0.3013,
                      top: -logoHeight * 0.636,
                    },
                  ]}
                />
              </View>
              <View
                style={[styles.vision, { top: visionTop }]}
                onLayout={({ nativeEvent: { layout: measured } }) => {
                  setVisionHeight(previous =>
                    Math.abs(previous - measured.height) < 0.5 ? previous : measured.height,
                  );
                }}
              >
                <Text style={styles.visionText}>{'FOR A\nSMARTER\nSRI LANKA'}</Text>
                <View style={styles.accent} />
              </View>
            </View>
            <View style={styles.card}>
              <View
                style={styles.form}
                onLayout={({ nativeEvent: { layout: measured } }) => {
                  setFormHeight(previous =>
                    Math.abs(previous - measured.height) < 0.5 ? previous : measured.height,
                  );
                  scheduleVisibilityCheck();
                }}
              >
                <View style={styles.formHeader}>
                  <Text accessibilityRole="header" style={styles.heading}>
                    Create account
                  </Text>
                  <Text style={styles.subtitle}>Sign up to start using GovAI</Text>
                </View>
                <Text accessibilityLiveRegion="polite" style={styles.notice}>
                  {notice}
                </Text>
                {fields.map((field, index) => {
                  const secret = field.key === 'password' || field.key === 'confirmPassword';
                  const shown = secret && revealed[field.key as 'password' | 'confirmPassword'];
                  const nameFont =
                    field.key === 'fullName' && /[\u0d80-\u0dff]/.test(values.fullName)
                      ? 'Yaldevi_400Regular'
                      : field.key === 'fullName' && /[\u0b80-\u0bff]/.test(values.fullName)
                        ? 'NotoSansTamil_400Regular'
                        : 'Inter_400Regular';
                  return (
                    <View
                      key={field.key}
                      ref={node => {
                        if (node) groupRefs.current[field.key] = node;
                        else delete groupRefs.current[field.key];
                      }}
                      onLayout={scheduleVisibilityCheck}
                      style={styles.fieldGroup}
                    >
                      <Text style={styles.label}>{field.label}</Text>
                      <View style={[styles.field, errors[field.key] && styles.invalid]}>
                        <Image source={field.icon} accessible={false} style={styles.fieldIcon} />
                        <TextInput
                          ref={node => {
                            if (node) inputRefs.current[field.key] = node;
                            else delete inputRefs.current[field.key];
                          }}
                          editable={!pending && !received}
                          value={values[field.key]}
                          onChangeText={value => {
                            operation.current = null;
                            setValues(previous => ({ ...previous, [field.key]: value }));
                          }}
                          placeholder={field.placeholder}
                          placeholderTextColor="#666666"
                          accessibilityLabel={`${field.label}. Required.${errors[field.key] ? ' ' + errors[field.key] : ''}`}
                          keyboardType={
                            field.key === 'email'
                              ? 'email-address'
                              : field.key === 'phone'
                                ? 'phone-pad'
                                : 'default'
                          }
                          autoCapitalize={field.key === 'fullName' ? 'words' : 'none'}
                          autoCorrect={false}
                          spellCheck={false}
                          autoComplete="off"
                          importantForAutofill="no"
                          secureTextEntry={secret && !shown}
                          returnKeyType={index === fields.length - 1 ? 'done' : 'next'}
                          submitBehavior="submit"
                          onSubmitEditing={() =>
                            index === fields.length - 1
                              ? submit()
                              : inputRefs.current[fields[index + 1].key]?.focus()
                          }
                          onFocus={() => focusField(field.key)}
                          onBlur={() => {
                            if (focused.current === field.key) {
                              focused.current = null;
                              cancelVisibilityCheck();
                            }
                          }}
                          style={[styles.input, { fontFamily: nameFont }]}
                        />
                        {secret && (
                          <Pressable
                            accessibilityRole="button"
                            accessibilityLabel={`${shown ? 'Hide' : 'Show'} ${field.label.toLowerCase()}`}
                            accessibilityState={{ selected: !!shown }}
                            style={styles.eye}
                            onPress={() => {
                              setRevealed(previous => ({
                                ...previous,
                                [field.key]: !previous[field.key as 'password' | 'confirmPassword'],
                              }));
                              scheduleVisibilityCheck();
                            }}
                          >
                            <Image
                              source={shown ? loginAssets.eye : registerAssets.eyeOff}
                              accessible={false}
                              style={styles.fieldIcon}
                            />
                          </Pressable>
                        )}
                      </View>
                      {errors[field.key] && (
                        <Text accessibilityLiveRegion="polite" style={styles.error}>
                          {errors[field.key]}
                        </Text>
                      )}
                    </View>
                  );
                })}
                <View style={styles.agreement}>
                  <Pressable
                    accessibilityRole="checkbox"
                    accessibilityState={{ checked: agreement }}
                    accessibilityLabel="Agreement demonstration. Optional; Terms of Service are unavailable."
                    onPress={() => setAgreement(previous => !previous)}
                    style={styles.checkboxTarget}
                  >
                    <View style={[styles.checkbox, agreement && styles.checked]}>
                      {agreement && <View accessible={false} style={styles.check} />}
                    </View>
                  </Pressable>
                  <View style={styles.agreementText}>
                    <Text style={styles.agreementCopy}>
                      {'I agree to the '}
                      <Text
                        accessible
                        accessibilityRole="link"
                        accessibilityState={{ disabled: true }}
                        accessibilityLabel="Terms of Service. Unavailable."
                        style={styles.terms}
                      >
                        Terms of Service (unavailable)
                      </Text>
                      {' and confirm the details above are correct.'}
                    </Text>
                    <Text style={styles.consentHint}>
                      Optional for this UI-only demonstration. Consent validation is pending.
                    </Text>
                  </View>
                </View>
                <Pressable
                  onPress={submit}
                  accessibilityRole="button"
                  disabled={pending}
                  accessibilityState={{ disabled: pending, busy: pending }}
                  accessibilityLabel={received ? 'Return to Log in' : 'Sign up'}
                  style={styles.primary}
                >
                  <Text style={[typography.action, styles.primaryText]}>
                    {pending ? 'Submitting...' : received ? 'Return to Log in' : 'Sign up'}
                  </Text>
                  <Image source={registerAssets.next} accessible={false} style={styles.nextIcon} />
                </Pressable>
                <View style={styles.loginRow}>
                  <Text style={styles.subtitle}>Already have an account?</Text>
                  <Pressable
                    onPress={leave}
                    accessibilityRole="link"
                    accessibilityLabel="Log in"
                    style={styles.loginLink}
                  >
                    <Text style={styles.linkText}>Log in</Text>
                  </Pressable>
                </View>
              </View>
            </View>
          </ScrollView>
        </View>
      </Screen>
    </View>
  );
}

const styles = StyleSheet.create({
  root: { flex: 1, backgroundColor: '#fff' },
  screen: { flex: 1 },
  topRibbon: { position: 'absolute', left: -9, top: -14 },
  skyline: { position: 'absolute', left: -164, bottom: -6, opacity: 0.45 },
  bottomRibbon: { position: 'absolute', right: -7, bottom: -8 },
  country: {
    position: 'absolute',
    bottom: 43,
    width: '100%',
    textAlign: 'center',
    fontFamily: 'Inter_400Regular',
    fontSize: 9,
    letterSpacing: 4,
    color: '#666',
  },
  scrollContent: { flexGrow: 1, gap: 12 },
  header: { width: '100%', alignItems: 'center', flexShrink: 0 },
  back: {
    position: 'absolute',
    left: 0,
    top: 0,
    width: layout.touchTarget,
    height: layout.touchTarget,
    justifyContent: 'center',
    alignItems: 'center',
    zIndex: 1,
  },
  backCircle: {
    width: 38,
    height: 38,
    borderRadius: 9999,
    backgroundColor: '#fff',
    alignItems: 'center',
    justifyContent: 'center',
    boxShadow: '0px 1px 4px rgba(0,0,0,0.04)',
  },
  backIcon: { width: 20, height: 20 },
  logoSlot: { overflow: 'hidden' },
  logo: { position: 'absolute' },
  vision: { position: 'absolute', right: 0, width: 86, alignItems: 'flex-end', gap: 6 },
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
    flexGrow: 1,
    paddingTop: 20,
    paddingBottom: 24,
    paddingHorizontal: 20,
    borderRadius: 24,
    borderWidth: 1,
    borderColor: '#f1f5f9',
    backgroundColor: '#fff',
    boxShadow: '0px 10px 22px rgba(0,0,0,0.1)',
  },
  form: { width: '100%', gap: 14 },
  formHeader: { gap: 4 },
  heading: { fontFamily: 'Inter_700Bold', fontSize: 20, lineHeight: 28, color: '#142033' },
  subtitle: { fontFamily: 'Inter_400Regular', fontSize: 13, lineHeight: 18, color: '#666' },
  notice: { fontFamily: 'Inter_400Regular', fontSize: 13, lineHeight: 18, color: '#8b1a2e' },
  fieldGroup: { width: '100%', gap: 6 },
  label: { fontFamily: 'Inter_500Medium', fontSize: 13, lineHeight: 18, color: '#142033' },
  field: {
    minHeight: layout.touchTarget,
    width: '100%',
    flexDirection: 'row',
    alignItems: 'center',
    gap: 10,
    paddingLeft: 14,
    paddingRight: 4,
    borderWidth: 1,
    borderColor: '#e5e7eb',
    borderRadius: 12,
    backgroundColor: '#fff',
    boxShadow: '0px 1px 4px rgba(0,0,0,0.04)',
  },
  fieldIcon: { width: 18, height: 18 },
  input: {
    flex: 1,
    minWidth: 0,
    paddingVertical: 12,
    paddingRight: 10,
    fontSize: 14,
    color: '#142033',
  },
  eye: {
    width: layout.touchTarget,
    minHeight: layout.touchTarget,
    justifyContent: 'center',
    alignItems: 'center',
  },
  invalid: { borderColor: '#a30f2d' },
  error: { fontFamily: 'Inter_400Regular', fontSize: 13, lineHeight: 18, color: '#a30f2d' },
  agreement: { flexDirection: 'row', alignItems: 'flex-start', gap: 4 },
  checkboxTarget: {
    width: layout.touchTarget,
    minHeight: layout.touchTarget,
    alignItems: 'center',
    justifyContent: 'center',
  },
  checkbox: {
    width: 22,
    height: 22,
    borderWidth: 1.5,
    borderColor: '#666',
    borderRadius: 2,
    alignItems: 'center',
    justifyContent: 'center',
  },
  checked: { backgroundColor: '#a30f2d', borderColor: '#a30f2d' },
  check: {
    width: 12,
    height: 7,
    borderLeftWidth: 2,
    borderBottomWidth: 2,
    borderColor: '#fff',
    marginTop: -3,
    transform: [{ rotate: '-45deg' }],
  },
  agreementText: { flex: 1, minWidth: 0, gap: 4 },
  agreementCopy: { fontFamily: 'Inter_400Regular', fontSize: 11, lineHeight: 16, color: '#444' },
  terms: { color: '#a30f2d', textDecorationLine: 'underline' },
  consentHint: { fontFamily: 'Inter_400Regular', fontSize: 11, lineHeight: 16, color: '#666' },
  primary: {
    minHeight: 52,
    paddingHorizontal: 24,
    paddingVertical: 16,
    flexDirection: 'row',
    alignItems: 'center',
    justifyContent: 'center',
    gap: 10,
    borderRadius: 14,
    backgroundColor: '#8b1a2e',
    boxShadow: '0px 6px 16px rgba(139,26,46,0.25)',
  },
  primaryText: {
    fontFamily: 'Inter_600SemiBold',
    color: '#fff',
    flexShrink: 1,
    textAlign: 'center',
  },
  nextIcon: { width: 16, height: 16 },
  loginRow: {
    flexDirection: 'row',
    flexWrap: 'wrap',
    alignItems: 'center',
    justifyContent: 'center',
    gap: 4,
  },
  loginLink: {
    minWidth: layout.touchTarget,
    minHeight: layout.touchTarget,
    justifyContent: 'center',
  },
  linkText: { fontFamily: 'Inter_500Medium', fontSize: 13, lineHeight: 18, color: '#a30f2d' },
});
