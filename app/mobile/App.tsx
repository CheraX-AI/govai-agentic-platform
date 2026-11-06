import { parseResetLink, type RecoveryReceipt } from './services/recoveryApi';
import { ResetPasswordForm } from './pages/auth/password-recovery/ResetPasswordPreviewScreen';
import { CheckEmailContent } from './components/auth/CheckEmailContent';
import { useNativeSession } from './auth/useNativeSession';
import { ChatHomeContent } from './components/ChatHomeContent';
import { useFonts } from 'expo-font';
import { useEffect, useRef, useState } from 'react';
import { Inter_600SemiBold } from '@expo-google-fonts/inter/600SemiBold';
import { Inter_700Bold } from '@expo-google-fonts/inter/700Bold';
import { Inter_400Regular } from '@expo-google-fonts/inter/400Regular';
import { Inter_500Medium } from '@expo-google-fonts/inter/500Medium';
import { StatusBar } from 'expo-status-bar';
import {
  Linking,
  BackHandler,
  Image,
  Keyboard,
  StyleSheet,
  Text,
  View,
  useWindowDimensions,
} from 'react-native';
import { SafeAreaProvider, useSafeAreaInsets } from 'react-native-safe-area-context';

import { WelcomeScreen, welcomeAssets } from './pages/onboarding/WelcomeScreen';
import { Screen } from './components/common/Screen';
import { OnboardingMeetScreen, onboardingAssets } from './pages/onboarding/OnboardingMeetScreen';
import { typography } from './layout';
import { LoginScreen, loginAssets } from './pages/auth/LoginScreen';
import { RegisterScreen, registerAssets } from './pages/auth/registration/RegisterScreen';
import {
  ForgotPasswordScreen,
  forgotPasswordAssets,
} from './pages/auth/password-recovery/ForgotPasswordScreen';
import { OnboardingPayScreen, payAssets } from './pages/onboarding/OnboardingPayScreen';
import { OnboardingTrackScreen, trackAssets } from './pages/onboarding/OnboardingTrackScreen';
import { OnboardingAskScreen } from './pages/onboarding/OnboardingAskScreen';
import {
  OnboardingDiscoverScreen,
  discoverAssets,
} from './pages/onboarding/OnboardingDiscoverScreen';

const assets = {
  topRibbon: require('./assets/figma/top-ribbon.png'),
  skyline: require('./assets/figma/skyline.png'),
  bottomRibbon: require('./assets/figma/bottom-ribbon.png'),
  logo: require('./assets/figma/logo.png'),
};

// Fit the startup artwork around safe areas while fonts and images load.

function LoadingScreen({ fontsLoaded }: { fontsLoaded: boolean }) {
  const { width, height, fontScale } = useWindowDimensions();
  const [visionHeight, setVisionHeight] = useState(0);
  const insets = useSafeAreaInsets();
  // Figma's 400 x 772 display includes simulated system chrome. The real OS
  // owns those areas. Fit the remaining artwork without stretching its assets.

  const scale = Math.min(
    1,
    Math.max(1, width - insets.left - insets.right) / 400,
    Math.max(1, height - insets.top - insets.bottom) / 712,
  );
  const canvasHeight = Math.max(712, (height - insets.top - insets.bottom) / scale);
  const middleOffset = (canvasHeight - 712) / 2;
  const fontFamily = fontsLoaded ? 'Inter_400Regular' : undefined;
  const logoWidth = 312 * scale;
  const logoHeight = 315 * scale;
  // Only measure intrinsic text, never the flexible slot that this controls.
  // The first render estimates the explicit three-line text; normal flow still
  // guarantees separation even before native measurement arrives.

  const intrinsicVisionHeight = visionHeight || 24 + 3 * 17 * fontScale + 6 + 2;
  const logoPadding = Math.max(0, (169 + middleOffset) * scale - 22 - intrinsicVisionHeight - 16);
  return (
    <View style={styles.root}>
      <StatusBar style="dark" />
      <View
        style={[
          styles.canvas,
          {
            left: insets.left + (width - insets.left - insets.right - 400) / 2,
            top: insets.top + (canvasHeight * scale - canvasHeight) / 2,
            height: canvasHeight,
            transform: [{ scale }],
          },
        ]}
      >
        <Image source={assets.topRibbon} accessible={false} style={styles.topRibbon} />
        <View style={[styles.skylineSlot, { top: canvasHeight - 353 }]} pointerEvents="none">
          <Image
            source={assets.skyline}
            accessible={false}
            resizeMode="stretch"
            style={styles.skyline}
          />
        </View>
        <Image source={assets.bottomRibbon} accessible={false} style={styles.bottomRibbon} />
      </View>
      <Screen contentStyle={styles.loadingContent}>
        <View
          style={styles.vision}
          onLayout={({ nativeEvent: { layout: measured } }) => {
            setVisionHeight(previous =>
              Math.abs(previous - measured.height) < 0.5 ? previous : measured.height,
            );
          }}
        >
          <Text
            style={[styles.visionText, { fontFamily: fontsLoaded ? 'Inter_500Medium' : undefined }]}
          >
            {'FOR A\nSMARTER\nSRI LANKA'}
          </Text>
          <View style={styles.accent} />
        </View>
        <View
          style={[
            styles.artworkSpace,
            { paddingTop: logoPadding, minHeight: Math.max(280, logoPadding + logoHeight) },
          ]}
        >
          <View
            accessible
            accessibilityRole="image"
            accessibilityLabel="GovAI. People, services, a smarter Sri Lanka."
            style={[
              styles.logoSlot,
              { width: logoWidth, height: logoHeight, transform: [{ translateX: 12 * scale }] },
            ]}
          >
            {/* Original source and nested crop ratios; only the slot joins flow. */}

            <Image
              source={assets.logo}
              accessible={false}
              resizeMode="stretch"
              style={[
                styles.logo,
                {
                  left: -logoWidth * 0.3013,
                  top: -logoHeight * 0.636,
                  width: logoWidth * 1.5417,
                  height: logoHeight * 2.7132,
                },
              ]}
            />
          </View>
        </View>
        <View
          accessibilityRole="progressbar"
          accessibilityLabel={'Building a better tomorrow. Loading.'}
          accessibilityValue={{ text: 'Loading' }}
          style={styles.loading}
        >
          <View style={styles.progress}>
            <View style={styles.loaded} />
            <View style={styles.remaining} />
          </View>
          <Text style={[typography.body, styles.loadingText, { fontFamily }]}>
            {'Building a better tomorrow...'}
          </Text>
        </View>
        <Text style={[styles.country, { fontFamily }]}>{'SRI LANKA'}</Text>
      </Screen>
    </View>
  );
}

export default function App() {
  return (
    <SafeAreaProvider>
      <Startup />
    </SafeAreaProvider>
  );
}

const startupAssets = [
  ...Object.values(assets),
  ...Object.values(welcomeAssets),
  ...Object.values(onboardingAssets),
  ...Object.values(discoverAssets),
  ...Object.values(trackAssets),
  ...Object.values(payAssets),
  ...Object.values(loginAssets),
  ...Object.values(registerAssets),
  ...Object.values(forgotPasswordAssets),
];

type OriginScreen = 'welcome' | 'meet' | 'discover' | 'ask' | 'track' | 'pay';

// Load startup resources and preserve the existing screen and preview navigation flow.

function Startup() {
  const [screen, setScreen] = useState<
    | OriginScreen
    | 'login'
    | 'register'
    | 'forgot'
    | 'demo-account'
    | 'demo-email'
    | 'demo-phone'
    | 'demo-chat'
  >('welcome');
  const navigation = useRef({ screen, loginVisit: 0 });
  if (navigation.current.screen !== screen) {
    if (screen === 'login') navigation.current.loginVisit++;
    navigation.current.screen = screen;
  }
  const loginVisit = navigation.current.loginVisit;
  const connectedDemo = __DEV__ && process.env.EXPO_PUBLIC_CONNECTED_UI_DEMO === '1';
  const nativeAuth = useNativeSession(!connectedDemo);
  const [recoveryReceipt, setRecoveryReceipt] = useState<RecoveryReceipt | null>(null);
  const [resetToken, setResetToken] = useState<string | null>(null);
  const [resetVisit, setResetVisit] = useState(0);
  useEffect(() => {
    if (connectedDemo) return;
    let active = true,
      eventReceived = false;
    const accept = (url: string) => {
      const token = parseResetLink(url, process.env.EXPO_PUBLIC_PASSWORD_RESET_URL);
      if (active && token) {
        setResetToken(token);
        setResetVisit(value => value + 1);
        setRecoveryReceipt(null);
      }
    };
    const subscription = Linking.addEventListener('url', event => {
      eventReceived = true;
      accept(event.url);
    });
    void Linking.getInitialURL()
      .then(url => {
        if (url && !eventReceived) accept(url);
      })
      .catch(() => undefined);
    return () => {
      active = false;
      subscription.remove();
    };
  }, [connectedDemo]);
  useEffect(() => {
    if (!recoveryReceipt || resetToken) return;
    const subscription = BackHandler.addEventListener('hardwareBackPress', () => {
      setRecoveryReceipt(null);
      setScreen('forgot');
      return true;
    });
    return () => subscription.remove();
  }, [recoveryReceipt, resetToken]);
  const [loginOrigin, setLoginOrigin] = useState<OriginScreen>('welcome');
  const [accountPreviewOpen, setAccountPreviewOpen] = useState(
    () => __DEV__ && process.env.EXPO_PUBLIC_ACCOUNT_CREATED_PREVIEW === '1',
  );
  const [checkEmailPreviewOpen, setCheckEmailPreviewOpen] = useState(
    () =>
      __DEV__ &&
      process.env.EXPO_PUBLIC_CHECK_EMAIL_PREVIEW === '1' &&
      process.env.EXPO_PUBLIC_ACCOUNT_CREATED_PREVIEW !== '1',
  );
  const [resetPasswordPreviewOpen, setResetPasswordPreviewOpen] = useState(
    () =>
      __DEV__ &&
      process.env.EXPO_PUBLIC_RESET_PASSWORD_PREVIEW === '1' &&
      process.env.EXPO_PUBLIC_ACCOUNT_CREATED_PREVIEW !== '1' &&
      process.env.EXPO_PUBLIC_CHECK_EMAIL_PREVIEW !== '1',
  );
  const [passwordUpdatedPreviewOpen, setPasswordUpdatedPreviewOpen] = useState(
    () =>
      __DEV__ &&
      process.env.EXPO_PUBLIC_PASSWORD_UPDATED_PREVIEW === '1' &&
      process.env.EXPO_PUBLIC_ACCOUNT_CREATED_PREVIEW !== '1' &&
      process.env.EXPO_PUBLIC_CHECK_EMAIL_PREVIEW !== '1' &&
      process.env.EXPO_PUBLIC_RESET_PASSWORD_PREVIEW !== '1',
  );
  const [chatHomePreviewOpen, setChatHomePreviewOpen] = useState(
    () =>
      __DEV__ &&
      process.env.EXPO_PUBLIC_CHAT_HOME_PREVIEW === '1' &&
      process.env.EXPO_PUBLIC_CONNECTED_UI_DEMO !== '1' &&
      process.env.EXPO_PUBLIC_ACCOUNT_CREATED_PREVIEW !== '1' &&
      process.env.EXPO_PUBLIC_CHECK_EMAIL_PREVIEW !== '1' &&
      process.env.EXPO_PUBLIC_RESET_PASSWORD_PREVIEW !== '1' &&
      process.env.EXPO_PUBLIC_PASSWORD_UPDATED_PREVIEW !== '1',
  );

  // Reset the login return destination when leaving this standalone preview.

  const closePasswordUpdatedPreview = (destination: 'welcome' | 'login') => {
    setPasswordUpdatedPreviewOpen(false);
    setLoginOrigin('welcome');
    setScreen(destination);
  };

  const exitResetPasswordPreview = () => {
    setResetPasswordPreviewOpen(false);
    setScreen('welcome');
  };

  const exitAccountPreview = () => {
    setAccountPreviewOpen(false);
    setScreen('welcome');
  };

  const exitCheckEmailPreview = (destination: 'welcome' | 'forgot' | 'login') => {
    setCheckEmailPreviewOpen(false);
    setScreen(destination);
  };

  // Remember which onboarding screen Android Back should return to from Login.

  const openLogin = (origin: OriginScreen) => {
    setLoginOrigin(origin);
    setScreen('login');
  };

  useEffect(() => {
    if (__DEV__ && accountPreviewOpen) {
      const subscription = BackHandler.addEventListener('hardwareBackPress', () => {
        exitAccountPreview();
        return true;
      });
      return () => subscription.remove();
    }
    if (__DEV__ && checkEmailPreviewOpen) {
      const subscription = BackHandler.addEventListener('hardwareBackPress', () => {
        exitCheckEmailPreview('forgot');
        return true;
      });
      return () => subscription.remove();
    }
    if (__DEV__ && resetPasswordPreviewOpen) return; // Preview owns keyboard-first Back and input clearing.
    if (__DEV__ && passwordUpdatedPreviewOpen) {
      const subscription = BackHandler.addEventListener('hardwareBackPress', () => {
        closePasswordUpdatedPreview('welcome');
        return true;
      });
      return () => subscription.remove();
    }
    if (resetToken || recoveryReceipt) return;
    if (nativeAuth.status === 'authenticated' || (connectedDemo && screen === 'demo-chat')) return; // Chat content owns keyboard-first Back.
    if (__DEV__ && chatHomePreviewOpen) return; // Chat preview owns keyboard-first Back and draft clearing.
    if (screen === 'welcome') return;
    const subscription = BackHandler.addEventListener('hardwareBackPress', () => {
      if (nativeAuth.status === 'signing-in') return true;
      if (Keyboard.isVisible()) {
        Keyboard.dismiss();
        return true;
      }
      if (connectedDemo && screen === 'demo-account') {
        setScreen('login');
        return true;
      }
      if (connectedDemo && (screen === 'demo-email' || screen === 'demo-phone')) {
        setScreen('forgot');
        return true;
      }
      if (screen === 'register' || screen === 'forgot') {
        setScreen('login');
        return true;
      }
      if (screen === 'login') {
        setScreen(loginOrigin);
        return true;
      }
      setScreen(
        screen === 'pay'
          ? 'track'
          : screen === 'track'
            ? 'ask'
            : screen === 'ask'
              ? 'discover'
              : screen === 'discover'
                ? 'meet'
                : 'welcome',
      );
      return true;
    });
    return () => subscription.remove();
  }, [
    resetToken,
    recoveryReceipt,
    nativeAuth.status,
    screen,
    loginOrigin,
    connectedDemo,
    accountPreviewOpen,
    checkEmailPreviewOpen,
    resetPasswordPreviewOpen,
    passwordUpdatedPreviewOpen,
    chatHomePreviewOpen,
  ]);
  const [fontsLoaded, fontError] = useFonts({
    Inter_400Regular,
    Inter_500Medium,
    Inter_600SemiBold,
    Inter_700Bold,
    Yaldevi_400Regular: require('./assets/figma/onboarding-ask/fonts/Yaldevi.ttf'),
    NotoSansTamil_400Regular: require('./assets/figma/onboarding-ask/fonts/NotoSansTamil.ttf'),
  });
  const [loadedAssets, setLoadedAssets] = useState<Set<number>>(() => new Set());
  const [assetError, setAssetError] = useState(false);
  const ready = fontsLoaded && loadedAssets.size === startupAssets.length && !assetError;
  // Replacement, not a pushed route: Android Back cannot revisit loading.

  if (ready && resetToken)
    return (
      <ResetPasswordForm
        key={resetVisit}
        token={resetToken}
        onComplete={nativeAuth.logout}
        onExit={() => {
          setResetToken(null);
          setScreen('login');
        }}
      />
    );
  if (ready && recoveryReceipt)
    return (
      <CheckEmailContent
        mode="live"
        receipt={recoveryReceipt}
        onBack={() => {
          setRecoveryReceipt(null);
          setScreen('forgot');
        }}
        onDifferentNic={() => {
          setRecoveryReceipt(null);
          setScreen('forgot');
        }}
        onLogin={() => {
          setRecoveryReceipt(null);
          setScreen('login');
        }}
      />
    );
  if (ready && nativeAuth.status === 'checking') return <LoadingScreen fontsLoaded={fontsLoaded} />;
  if (ready && nativeAuth.status === 'authenticated')
    return (
      <ChatHomeContent
        onSendChat={nativeAuth.chat}
        mode="authenticated"
        onExit={() => {
          setScreen('login');
          void nativeAuth.logout();
        }}
      />
    );
  if (__DEV__) {
    if (ready && accountPreviewOpen && process.env.EXPO_PUBLIC_ACCOUNT_CREATED_PREVIEW === '1') {
      // Keep require inside a standalone development branch so Metro excludes
      // this dependency from the production graph, not just the active route.

      const { AccountCreatedPreviewScreen } =
        require('./pages/auth/registration/AccountCreatedPreviewScreen') as typeof import('./pages/auth/registration/AccountCreatedPreviewScreen');
      return <AccountCreatedPreviewScreen onExit={exitAccountPreview} />;
    }
    if (ready && checkEmailPreviewOpen && process.env.EXPO_PUBLIC_CHECK_EMAIL_PREVIEW === '1') {
      // Keep the module and its exclusive asset outside the production graph.

      const { CheckEmailPreviewScreen } =
        require('./pages/auth/password-recovery/CheckEmailPreviewScreen') as typeof import('./pages/auth/password-recovery/CheckEmailPreviewScreen');
      return (
        <CheckEmailPreviewScreen
          onBack={() => exitCheckEmailPreview('forgot')}
          onDifferentNic={() => exitCheckEmailPreview('forgot')}
          onLogin={() => exitCheckEmailPreview('login')}
          onExit={() => exitCheckEmailPreview('welcome')}
        />
      );
    }
  }
  if (__DEV__) {
    if (
      ready &&
      resetPasswordPreviewOpen &&
      process.env.EXPO_PUBLIC_RESET_PASSWORD_PREVIEW === '1'
    ) {
      // A development-only dependency, excluded from the release graph.

      const { ResetPasswordPreviewScreen } =
        require('./pages/auth/password-recovery/ResetPasswordPreviewScreen') as typeof import('./pages/auth/password-recovery/ResetPasswordPreviewScreen');
      return <ResetPasswordPreviewScreen onExit={exitResetPasswordPreview} />;
    }
  }
  if (__DEV__) {
    if (
      ready &&
      passwordUpdatedPreviewOpen &&
      process.env.EXPO_PUBLIC_PASSWORD_UPDATED_PREVIEW === '1'
    ) {
      // Keep this preview and its exclusive shield outside the release graph.

      const { PasswordUpdatedPreviewScreen } =
        require('./pages/auth/password-recovery/PasswordUpdatedPreviewScreen') as typeof import('./pages/auth/password-recovery/PasswordUpdatedPreviewScreen');
      return <PasswordUpdatedPreviewScreen onLogin={() => closePasswordUpdatedPreview('login')} />;
    }
  }
  if (__DEV__) {
    if (ready && chatHomePreviewOpen && process.env.EXPO_PUBLIC_CHAT_HOME_PREVIEW === '1') {
      const { ChatHomePreviewScreen } =
        require('./pages/ChatHomePreviewScreen') as typeof import('./pages/ChatHomePreviewScreen');
      return (
        <ChatHomePreviewScreen
          onExit={() => {
            setChatHomePreviewOpen(false);
            setScreen('welcome');
          }}
        />
      );
    }
    if (ready && connectedDemo && screen === 'demo-chat') {
      const { ChatHomeDemoScreen } =
        require('./pages/ChatHomePreviewScreen') as typeof import('./pages/ChatHomePreviewScreen');
      return (
        <ChatHomeDemoScreen
          onExit={() => {
            if (
              navigation.current.screen !== 'demo-chat' ||
              navigation.current.loginVisit !== loginVisit
            )
              return;
            setScreen(current => (current === 'demo-chat' ? 'login' : current));
          }}
        />
      );
    }
    if (ready && connectedDemo && screen === 'demo-account') {
      const { AccountCreatedDemoScreen } =
        require('./pages/auth/registration/AccountCreatedPreviewScreen') as typeof import('./pages/auth/registration/AccountCreatedPreviewScreen');
      return (
        <AccountCreatedDemoScreen
          onContinue={() => setScreen(current => (current === 'demo-account' ? 'login' : current))}
        />
      );
    }
    if (ready && connectedDemo && screen === 'demo-phone') {
      const { CheckPhoneDemoScreen } =
        require('./pages/auth/password-recovery/CheckPhoneDemoScreen') as typeof import('./pages/auth/password-recovery/CheckPhoneDemoScreen');
      const returnToForgot = () =>
        setScreen(current => (current === 'demo-phone' ? 'forgot' : current));
      return (
        <CheckPhoneDemoScreen
          onBack={returnToForgot}
          onDifferentNic={returnToForgot}
          onLogin={() => setScreen(current => (current === 'demo-phone' ? 'login' : current))}
        />
      );
    }
    if (ready && connectedDemo && screen === 'demo-email') {
      const { CheckEmailDemoScreen } =
        require('./pages/auth/password-recovery/CheckEmailPreviewScreen') as typeof import('./pages/auth/password-recovery/CheckEmailPreviewScreen');
      const returnToForgot = () =>
        setScreen(current => (current === 'demo-email' ? 'forgot' : current));
      return (
        <CheckEmailDemoScreen
          onBack={returnToForgot}
          onDifferentNic={returnToForgot}
          onLogin={() => setScreen(current => (current === 'demo-email' ? 'login' : current))}
        />
      );
    }
  }
  if (ready && screen === 'register')
    return (
      <RegisterScreen
        onLogin={() => setScreen('login')}
        onDemoValidated={
          connectedDemo
            ? () => setScreen(current => (current === 'register' ? 'demo-account' : current))
            : undefined
        }
      />
    );
  if (ready && screen === 'forgot')
    return (
      <ForgotPasswordScreen
        onRecoveryRequested={!connectedDemo ? setRecoveryReceipt : undefined}
        onLogin={() => setScreen('login')}
        onDemoValidated={
          connectedDemo
            ? (method: 'email' | 'phone' = 'email') =>
                setScreen(current =>
                  current === 'forgot'
                    ? method === 'phone'
                      ? 'demo-phone'
                      : 'demo-email'
                    : current,
                )
            : undefined
        }
      />
    );
  if (ready && screen === 'login')
    return (
      <LoginScreen
        onAuthenticate={!connectedDemo ? nativeAuth.login : undefined}
        sessionNotice={nativeAuth.error || undefined}
        onDemoValidated={
          connectedDemo
            ? () => {
                if (
                  navigation.current.screen !== 'login' ||
                  navigation.current.loginVisit !== loginVisit
                )
                  return;
                navigation.current.screen = 'demo-chat';
                setScreen('demo-chat');
              }
            : undefined
        }
        onBack={() => setScreen(loginOrigin)}
        onSignUp={() => setScreen('register')}
        onForgotPassword={() => setScreen('forgot')}
      />
    );
  if (ready && screen === 'pay')
    return (
      <OnboardingPayScreen onLogin={() => openLogin('pay')} onBack={() => setScreen('track')} />
    );
  if (ready && screen === 'track')
    return (
      <OnboardingTrackScreen
        onLogin={() => openLogin('track')}
        onBack={() => setScreen('ask')}
        onNext={() => setScreen('pay')}
      />
    );
  if (ready && screen === 'ask')
    return (
      <OnboardingAskScreen
        onLogin={() => openLogin('ask')}
        onBack={() => setScreen('discover')}
        onNext={() => setScreen('track')}
      />
    );
  if (ready && screen === 'discover')
    return (
      <OnboardingDiscoverScreen
        onLogin={() => openLogin('discover')}
        onBack={() => setScreen('meet')}
        onNext={() => setScreen('ask')}
      />
    );
  if (ready)
    return screen === 'meet' ? (
      <OnboardingMeetScreen
        onLogin={() => openLogin('meet')}
        onBack={() => setScreen('welcome')}
        onNext={() => setScreen('discover')}
      />
    ) : (
      <WelcomeScreen onLogin={() => openLogin('welcome')} onGetStarted={() => setScreen('meet')} />
    );
  return (
    <View style={styles.root}>
      {fontError || assetError ? (
        <Screen contentStyle={{ justifyContent: 'center' }}>
          <StatusBar style="dark" />
          <Text
            accessibilityRole="alert"
            accessibilityLiveRegion="assertive"
            style={[typography.body, { color: '#8b1a2e', padding: 16, textAlign: 'center' }]}
          >
            {'Unable to load startup resources. Please reload the app.'}{' '}
          </Text>
        </Screen>
      ) : (
        <LoadingScreen fontsLoaded={fontsLoaded} />
      )}
      <View
        pointerEvents="none"
        accessibilityElementsHidden
        importantForAccessibility="no-hide-descendants"
        style={{ position: 'absolute', width: 1, height: 1, opacity: 0 }}
      >
        {startupAssets.map((source, index) => (
          <Image
            key={index}
            source={source}
            fadeDuration={0}
            style={{ width: 1, height: 1 }}
            onLoad={() => setLoadedAssets(previous => new Set(previous).add(index))}
            onError={() => setAssetError(true)}
          />
        ))}
      </View>
    </View>
  );
}

const styles = StyleSheet.create({
  root: { flex: 1, backgroundColor: '#fff', overflow: 'hidden' },
  canvas: { position: 'absolute', width: 400, overflow: 'hidden', backgroundColor: '#fff' },
  topRibbon: { position: 'absolute', left: -15, top: -42, width: 272, height: 296 },
  bottomRibbon: { position: 'absolute', left: 170, bottom: 0, width: 237, height: 167 },
  skylineSlot: {
    position: 'absolute',
    left: -164,
    width: 723,
    height: 397,
    opacity: 0.45,
    overflow: 'hidden',
  },
  skyline: {
    position: 'absolute',
    left: 723 * 0.2259,
    top: -397 * 0.094,
    width: 723 * 0.5506,
    height: 397 * 1.1086,
  },
  vision: { alignSelf: 'flex-end', alignItems: 'flex-end', gap: 6, paddingTop: 24 },
  visionText: { color: '#666', fontSize: 12, lineHeight: 17, letterSpacing: 2, textAlign: 'right' },
  accent: { width: 36, height: 2, backgroundColor: '#f5a915' },
  logoSlot: { alignSelf: 'center', overflow: 'hidden' },
  logo: { position: 'absolute' },
  loadingContent: { gap: 16 },
  artworkSpace: { flexGrow: 1, flexShrink: 0 },
  loading: { alignItems: 'center', gap: 10, backgroundColor: '#fff', padding: 12 },
  progress: { flexDirection: 'row', width: 102, height: 3, overflow: 'hidden' },
  loaded: { width: 51, backgroundColor: '#8b1a2e' },
  remaining: { width: 51, backgroundColor: '#e5e7eb' },
  loadingText: { color: '#666', textAlign: 'center' },
  country: {
    paddingTop: 40,
    color: '#9ca3af',
    fontSize: 12,
    lineHeight: 17,
    letterSpacing: 4,
    textAlign: 'center',
  },
});
