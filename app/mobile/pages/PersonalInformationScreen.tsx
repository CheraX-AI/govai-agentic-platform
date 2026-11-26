import { useEffect, useRef, useState } from 'react';
import { ActivityIndicator, Alert, BackHandler, Image, Keyboard, KeyboardAvoidingView, Platform, Pressable, ScrollView, StyleSheet, Text, View, useWindowDimensions } from 'react-native';
import { useSafeAreaInsets } from 'react-native-safe-area-context';
import { StatusBar } from 'expo-status-bar';
import { useCitizenProfile } from '../auth/useCitizenProfile';
import type { ProfileApi } from '../services/profileApi';
import { PersonalInformationForm } from '../components/PersonalInformationForm';
import { ProfilePhotoCard } from '../components/ProfilePhotoCard';
import { ChatBottomNavigation } from '../components/ChatBottomNavigation';
import { layout } from '../layout';

const navigationAssets = {
  home: require('../assets/figma/chat-home/home.png'), services: require('../assets/figma/chat-home/services.png'),
  requests: require('../assets/figma/chat-home/requests.png'), profile: require('../assets/figma/chat-home/profile.png'),
};
export function PersonalInformationScreen({ api, onBack }: { api: ProfileApi; onBack: () => void; }) {
  const model = useCitizenProfile(api);
  const insets = useSafeAreaInsets();
  const scroll = useRef<ScrollView>(null);
  const { height, fontScale } = useWindowDimensions();
  const [keyboardVisible, setKeyboardVisible] = useState(false);
  const inlineChrome = height < 600 || fontScale > 1.4;
  useEffect(() => {
    const shown = Keyboard.addListener('keyboardDidShow', () => setKeyboardVisible(true));
    const hidden = Keyboard.addListener('keyboardDidHide', () => setKeyboardVisible(false));
    return () => { shown.remove(); hidden.remove(); };
  }, []);
  function leave() {
    if (Keyboard.isVisible()) { Keyboard.dismiss(); return; }
    if (model.dirty || model.selectedPhoto || model.busy) {
      Alert.alert('Leave personal information?', model.busy ? 'A request may still finish. Reopen your profile to check its saved state.' : 'Your unsaved changes will be discarded.', [
        { text: 'Stay', style: 'cancel' }, { text: 'Leave', style: 'destructive', onPress: onBack },
      ]);
    } else onBack();
  }
  useEffect(() => {
    const subscription = BackHandler.addEventListener('hardwareBackPress', () => { leave(); return true; });
    return () => subscription.remove();
  });
  const header = (
    <View style={styles.header}>
      <Pressable accessibilityRole="button" accessibilityLabel="Back to Home" onPress={leave} style={styles.back}><Image source={require('../assets/figma/register/back.png')} accessible={false} style={styles.backIcon} /></Pressable>
      <Text accessibilityRole="header" style={styles.title}>Personal Information</Text><View style={styles.headerBalance} />
    </View>
  );
  const saveAction = (
    <View style={styles.footer}>
      <View style={styles.formWidth}><Pressable accessibilityRole="button" accessibilityState={{ disabled: model.busy || !model.dirty || model.needsReload }}
        disabled={model.busy || !model.dirty || model.needsReload} onPress={() => { Keyboard.dismiss(); void model.save(); }} style={styles.save}>
        <Text style={styles.saveText}>{model.busy ? 'Please wait...' : 'Save changes'}</Text>
      </Pressable></View>
    </View>
  );
  // Safe-area edges belong to this screen. iOS keyboard padding has one owner;
  // Android uses activity resize mode rather than double compensation.
  return <KeyboardAvoidingView style={styles.root} behavior={Platform.OS === 'ios' ? 'padding' : undefined}>
    <StatusBar style="dark" />
    <View style={[styles.safe, { paddingTop: insets.top, paddingBottom: insets.bottom, paddingLeft: insets.left, paddingRight: insets.right }]}>
      {!inlineChrome && header}
      <ScrollView ref={scroll} style={styles.scroll} contentContainerStyle={styles.content} keyboardShouldPersistTaps="handled" keyboardDismissMode="on-drag"
        automaticallyAdjustKeyboardInsets={false} automaticallyAdjustContentInsets={false} contentInsetAdjustmentBehavior="never">
        <View style={styles.formWidth}>
          {inlineChrome && header}
          {!model.profile || !model.draft ? <View style={styles.loading}>
            {model.error ? <><Text accessibilityRole="alert" style={styles.error}>{model.error}</Text><Pressable accessibilityRole="button" onPress={model.retryLoad} style={styles.outline}><Text style={styles.outlineText}>Try again</Text></Pressable></> : <><ActivityIndicator color="#891b30" /><Text accessibilityLiveRegion="polite" style={styles.body}>Loading your personal information...</Text></>}
          </View> : <>
            {!!model.error && <Text accessibilityRole="alert" style={[styles.message, styles.error]}>{model.error}</Text>}
            {!!model.notice && <Text accessibilityLiveRegion="polite" style={[styles.message, styles.success]}>{model.notice}</Text>}
            {model.needsReload && <View style={styles.reconcile}><Text style={styles.body}>Your edits are still here. Load the latest profile before continuing.</Text>
              <Pressable accessibilityRole="button" disabled={model.busy} onPress={() => void model.reload(true)} style={styles.outline}><Text style={styles.outlineText}>Load latest and keep my edits</Text></Pressable>
              <Pressable accessibilityRole="button" disabled={model.busy} onPress={() => void model.reload(false)} style={styles.outline}><Text style={styles.outlineText}>Discard edits and load latest</Text></Pressable>
            </View>}
            <ProfilePhotoCard model={model} />
            <PersonalInformationForm model={model} onFieldFocus={target => scroll.current?.scrollResponderScrollNativeHandleToKeyboard(target, 20, true)} />
            {(model.dirty || model.selectedPhoto) && <Pressable accessibilityRole="button" disabled={model.busy} onPress={model.cancel} style={styles.outline}><Text style={styles.outlineText}>Cancel changes</Text></Pressable>}
          </>}
          {inlineChrome && saveAction}
        </View>
      </ScrollView>
      {!inlineChrome && saveAction}
      {!keyboardVisible && <ChatBottomNavigation assets={navigationAssets} selectedTab="profile" onHome={leave} onProfile={() => { }} />}
    </View>
  </KeyboardAvoidingView>;
}
const styles = StyleSheet.create({
  root: { flex: 1, backgroundColor: '#f7f9fb' }, safe: { flex: 1 },
  header: { flexDirection: 'row', alignItems: 'center', gap: 8, backgroundColor: '#fff', paddingHorizontal: 8, paddingVertical: 6 },
  back: { minWidth: layout.touchTarget, minHeight: layout.touchTarget, alignItems: 'center', justifyContent: 'center' }, backIcon: { width: 24, height: 24, resizeMode: 'contain' },
  title: { flex: 1, fontFamily: 'Inter_700Bold', color: '#142033', fontSize: 16, lineHeight: 23, textAlign: 'center' }, headerBalance: { width: layout.touchTarget },
  scroll: { flex: 1 }, content: { paddingHorizontal: 16, paddingTop: 8, paddingBottom: 24, flexGrow: 1 },
  formWidth: { width: '100%', maxWidth: layout.contentWidth, alignSelf: 'center', gap: 14 },
  loading: { gap: 16, paddingVertical: 40 }, body: { fontFamily: 'Inter_400Regular', fontSize: 14, lineHeight: 22, color: '#142033' },
  message: { padding: 12, borderRadius: 12, fontFamily: 'Inter_400Regular', fontSize: 14, lineHeight: 22 },
  error: { color: '#891b30', backgroundColor: '#fff0f3', fontFamily: 'Inter_400Regular', fontSize: 14, lineHeight: 22 }, success: { color: '#24543b', backgroundColor: '#edf8f1' },
  reconcile: { backgroundColor: '#fff8e9', padding: 14, borderRadius: 14, gap: 12 },
  outline: { minHeight: layout.touchTarget, padding: 12, borderRadius: 12, borderWidth: 1, borderColor: '#891b30', alignItems: 'center', justifyContent: 'center' },
  outlineText: { fontFamily: 'Inter_600SemiBold', color: '#891b30', fontSize: 14, lineHeight: 22, textAlign: 'center' },
  footer: { paddingHorizontal: 16, paddingVertical: 10, backgroundColor: '#fff', borderTopWidth: 1, borderTopColor: '#edf0f4' },
  save: { minHeight: layout.touchTarget, backgroundColor: '#891b30', borderRadius: 14, alignItems: 'center', justifyContent: 'center', padding: 12 },
  saveText: { fontFamily: 'Inter_600SemiBold', color: '#fff', fontSize: 16, lineHeight: 23, textAlign: 'center' },
});
